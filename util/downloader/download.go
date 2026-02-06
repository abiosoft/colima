package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/shautil"
)

type (
	hostActions  = environment.HostActions
	guestActions = environment.GuestActions
)

// Request is download request
type Request struct {
	URL string // request URL
	SHA *SHA   // shasum url
}

// DownloadToGuest downloads file at url and saves it in the destination.
//
// In the implementation, the file is downloaded (and cached) on the host, but copied to the desired
// destination for the guest.
// filename must be an absolute path and a directory on the guest that does not require root access.
func DownloadToGuest(host hostActions, guest guestActions, r Request, filename string) error {
	// if file is on the filesystem, no need for download. A copy suffices
	if strings.HasPrefix(r.URL, "/") {
		return guest.RunQuiet("cp", r.URL, filename)
	}

	cacheFile, err := Download(host, r)
	if err != nil {
		return err
	}

	return guest.RunQuiet("cp", cacheFile, filename)
}

// Download downloads file at url and returns the location of the downloaded file.
func Download(host hostActions, r Request) (string, error) {
	d := downloader{}

	if !d.hasCache(r.URL) {
		if err := d.downloadFile(r); err != nil {
			return "", err
		}
	}

	return CacheFilename(r.URL), nil
}

type downloader struct{}

// CacheFilename returns the computed filename for the url.
func CacheFilename(url string) string {
	return filepath.Join(config.CacheDir(), "caches", shautil.SHA256(url).String())
}

func (d downloader) cacheDownloadingFileName(url string) string {
	return CacheFilename(url) + ".downloading"
}

func (d downloader) resumeInfoPath(url string) string {
	return CacheFilename(url) + ".resume"
}

func (d downloader) downloadFile(r Request) (err error) {
	cacheDownloadingFilename := d.cacheDownloadingFileName(r.URL)

	// create cache directory
	cacheDir := filepath.Dir(cacheDownloadingFilename)
	if err := os.MkdirAll(cacheDir, 0755); err != nil {
		return fmt.Errorf("error preparing cache dir: %w", err)
	}

	// check for existing partial download and resume info
	var resumeInfo ResumeInfo
	resumeInfoPath := d.resumeInfoPath(r.URL)
	if data, err := os.ReadFile(resumeInfoPath); err == nil {
		_ = json.Unmarshal(data, &resumeInfo)
	}

	// get existing file size for resume
	var existingSize int64
	if stat, err := os.Stat(cacheDownloadingFilename); err == nil {
		existingSize = stat.Size()
	}

	// create HTTP client
	client := NewHTTPClient()

	// use a long timeout for large files (2 hours)
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Hour)
	defer cancel()

	// get final URL (follows redirects)
	finalURL, err := client.GetFinalURL(ctx, r.URL)
	if err != nil {
		return fmt.Errorf("error resolving download URL '%s': %w", r.URL, err)
	}

	// download the file
	result, err := client.Download(ctx, DownloadOptions{
		URL:            finalURL,
		DestPath:       cacheDownloadingFilename,
		ExpectedETag:   resumeInfo.ETag,
		ResumeFromByte: existingSize,
		ShowProgress:   true,
	})
	if err != nil {
		// save resume info for next attempt if we have ETag
		if result != nil && result.ETag != "" {
			d.saveResumeInfo(r.URL, result.ETag, existingSize)
		}
		return fmt.Errorf("error downloading '%s': %w", path.Base(r.URL), err)
	}

	// clean up resume info on successful download
	_ = os.Remove(resumeInfoPath)

	// validate download if SHA is present
	if r.SHA != nil {
		if err := r.SHA.validateDownload(r.URL, cacheDownloadingFilename); err != nil {
			// move file to allow subsequent re-download
			_ = os.Rename(cacheDownloadingFilename, cacheDownloadingFilename+".invalid")
			return fmt.Errorf("error validating SHA sum for '%s': %w", path.Base(r.URL), err)
		}
	}

	// move completed download to final location
	if err := os.Rename(cacheDownloadingFilename, CacheFilename(r.URL)); err != nil {
		return fmt.Errorf("error finalizing download: %w", err)
	}

	return nil
}

func (d downloader) saveResumeInfo(url, etag string, bytesWritten int64) {
	info := ResumeInfo{ETag: etag, BytesWritten: bytesWritten}
	data, _ := json.Marshal(info)
	_ = os.WriteFile(d.resumeInfoPath(url), data, 0644)
}

func (d downloader) hasCache(url string) bool {
	_, err := os.Stat(CacheFilename(url))
	return err == nil
}
