package downloader

import (
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/osutil"
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

// FileDownloader is the interface for downloading files
type FileDownloader interface {
	Download(r Request, destPath string) error
}

// fileDownloader is the configured downloader implementation
var fileDownloader FileDownloader = &nativeDownloader{}

// SetDownloader sets the downloader implementation based on the provided type.
// The value should be validated before calling this function.
func SetDownloader(v string) {
	if v == DownloaderCurl {
		fileDownloader = &curlDownloader{}
	} else {
		fileDownloader = &nativeDownloader{}
	}
}

func init() {
	// check environment variable for default downloader
	if v := osutil.EnvVar(envDownloader).Val(); v != "" {
		if d, err := ValidateDownloader(v); err == nil {
			SetDownloader(d)
		}
	}
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

	if err := fileDownloader.Download(r, cacheDownloadingFilename); err != nil {
		return err
	}

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
