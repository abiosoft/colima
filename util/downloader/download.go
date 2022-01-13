package downloader

import (
	"crypto/sha256"
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/terminal"
	"os"
	"path/filepath"
)

// Download downloads file at url and saves it in the destination.
//
// In the implementation, the file is downloaded (and cached) on the host, but copied to the desired
// destination for the guest.
// fileName must be a directory on the guest that does not require root access.
func Download(host environment.HostActions, guest environment.GuestActions, url, fileName string) error {
	d := downloader{
		host:  host,
		guest: guest,
	}

	if !d.hasCache(url) {
		if err := d.downloadFile(url); err != nil {
			return fmt.Errorf("error downloading '%s': %w", url, err)
		}
	}

	return guest.RunQuiet("cp", d.cacheFileName(url), fileName)
}

type downloader struct {
	host  environment.HostActions
	guest environment.GuestActions
}

func (d downloader) cacheFileName(url string) string {
	return filepath.Join(config.CacheDir(), "caches", sha256Hash(url))
}

func (d downloader) cacheDownloadingFileName(url string) string {
	return d.cacheFileName(url) + ".downloading"
}

func (d downloader) downloadFile(url string) (err error) {
	// save to a temporary file initially before renaming to the desired file after successful download
	// this prevents having a corrupt file
	cacheFileName := d.cacheDownloadingFileName(url)
	if err := d.host.RunQuiet("mkdir", "-p", filepath.Dir(cacheFileName)); err != nil {
		return fmt.Errorf("error preparing cache dir: %w", err)
	}

	// get rid of curl's initial progress bar by getting the redirect url directly.
	downloadURL, err := d.host.RunOutput("curl", "-Ls", "-o", "/dev/null", "-w", "%{url_effective}", url)
	if err != nil {
		return fmt.Errorf("error retrieving redirect url: %w", err)
	}

	// ask curl to resume previous download if possible "-C -"
	if err := d.host.RunInteractive("curl", "-L", "-#", "-C", "-", "-o", cacheFileName, downloadURL); err != nil {
		return err
	}
	// clear curl progress line
	terminal.ClearLine()

	return d.host.RunQuiet("mv", d.cacheDownloadingFileName(url), d.cacheFileName(url))

}

func (d downloader) hasCache(url string) bool {
	_, err := os.Stat(d.cacheFileName(url))
	return err == nil
}

func sha256Hash(s string) string {
	sum := sha256.Sum256([]byte(s))
	return fmt.Sprintf("%x", sum)
}
