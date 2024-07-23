package downloader

import (
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/shautil"
	"github.com/abiosoft/colima/util/terminal"
)

type (
	hostActions  = environment.HostActions
	guestActions = environment.GuestActions
)

type SHA struct {
	URL    string // url to download the shasum file (if Digest is empty)
	Size   int    // one of 256 or 512
	Digest string // shasum
}

func (s SHA) validate(host hostActions, url, cacheFilename string) error {
	if s.URL == "" && s.Digest == "" {
		return fmt.Errorf("error validating SHA: one of Digest or URL must be set")
	}

	if s.Digest != "" {
		s.Digest = strings.TrimPrefix(s.Digest, fmt.Sprintf("sha%d:", s.Size))
	}

	filename := func() string {
		if url == "" {
			return ""
		}
		split := strings.Split(url, "/")
		return split[len(split)-1]
	}()
	dir, cacheFilename := filepath.Split(cacheFilename)

	var script string

	if s.Digest == "" {
		script = strings.NewReplacer(
			"{dir}", dir,
			"{url}", s.URL,
			"{filename}", filename,
			"{size}", strconv.Itoa(s.Size),
			"{cache_filename}", cacheFilename,
		).Replace(
			`cd {dir} && echo "$(curl -sL {url} | grep '  {filename}$' | awk -F' ' '{print $1}')  {cache_filename}" | shasum -a {size} --check --status`,
		)
	} else {
		script = strings.NewReplacer(
			"{dir}", dir,
			"{digest}", s.Digest,
			"{filename}", filename,
			"{size}", strconv.Itoa(s.Size),
			"{cache_filename}", cacheFilename,
		).Replace(
			`cd {dir} && echo "{digest}  {cache_filename}" | shasum -a {size} --check --status`,
		)
	}

	return host.Run("sh", "-c", script)
}

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
	d := downloader{
		host: host,
	}

	if !d.hasCache(r.URL) {
		if err := d.downloadFile(r); err != nil {
			return "", fmt.Errorf("error downloading '%s': %w", r.URL, err)
		}
	}

	return d.cacheFilename(r.URL), nil
}

type downloader struct {
	host hostActions
}

func (d downloader) cacheFilename(url string) string {
	return filepath.Join(config.CacheDir(), "caches", shautil.SHA256(url).String())
}

func (d downloader) cacheDownloadingFileName(url string) string {
	return d.cacheFilename(url) + ".downloading"
}

func (d downloader) downloadFile(r Request) (err error) {
	// save to a temporary file initially before renaming to the desired file after successful download
	// this prevents having a corrupt file
	cacheDownloadingFilename := d.cacheDownloadingFileName(r.URL)
	if err := d.host.RunQuiet("mkdir", "-p", filepath.Dir(cacheDownloadingFilename)); err != nil {
		return fmt.Errorf("error preparing cache dir: %w", err)
	}

	// get rid of curl's initial progress bar by getting the redirect url directly.
	downloadURL, err := d.host.RunOutput("curl", "-Ls", "-o", "/dev/null", "-w", "%{url_effective}", r.URL)
	if err != nil {
		return fmt.Errorf("error retrieving redirect url: %w", err)
	}

	// ask curl to resume previous download if possible "-C -"
	if err := d.host.RunInteractive("curl", "-L", "-#", "-C", "-", "-o", cacheDownloadingFilename, downloadURL); err != nil {
		return err
	}
	// clear curl progress line
	terminal.ClearLine()

	// validate download if sha is present
	if r.SHA != nil {
		if err := r.SHA.validate(d.host, r.URL, cacheDownloadingFilename); err != nil {

			// move file to allow subsequent re-download
			// error discarded, would not be actioned anyways
			_ = d.host.RunQuiet("mv", cacheDownloadingFilename, cacheDownloadingFilename+".invalid")

			return fmt.Errorf("error validating SHA sum for '%s': %w", path.Base(r.URL), err)
		}
	}

	return d.host.RunQuiet("mv", cacheDownloadingFilename, d.cacheFilename(r.URL))
}

func (d downloader) hasCache(url string) bool {
	_, err := os.Stat(d.cacheFilename(url))
	return err == nil
}
