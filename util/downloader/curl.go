package downloader

import (
	"fmt"
	"os"
	"os/exec"
	"path"
	"strings"

	"github.com/abiosoft/colima/util/terminal"
)

const (
	// DownloaderNative uses Go's native HTTP client
	DownloaderNative = "native"
	// DownloaderCurl uses the curl command (honors .curlrc)
	DownloaderCurl = "curl"

	envDownloader = "COLIMA_DOWNLOADER"
)

// ValidateDownloader validates the downloader value (case-insensitive).
// Returns the normalized value or an error if invalid.
func ValidateDownloader(v string) (string, error) {
	switch strings.ToLower(v) {
	case DownloaderNative:
		return DownloaderNative, nil
	case DownloaderCurl:
		return DownloaderCurl, nil
	default:
		return "", fmt.Errorf("invalid downloader %q: must be one of %s, %s", v, DownloaderNative, DownloaderCurl)
	}
}

// curlDownloader handles downloads using the curl command
type curlDownloader struct{}

// Download downloads a file using curl
func (c *curlDownloader) Download(r Request, destPath string) error {
	// check if curl is available
	if _, err := exec.LookPath("curl"); err != nil {
		return fmt.Errorf("curl not found in PATH: %w", err)
	}

	args := []string{
		"-fSL",    // fail on HTTP errors, show errors, follow redirects
		"-C", "-", // resume if possible (auto-detect offset)
		"--progress-bar", // show progress bar
		"-o", destPath,   // output file
		r.URL,
	}

	cmd := exec.Command("curl", args...)
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("curl download failed for '%s': %w", path.Base(r.URL), err)
	}

	terminal.ClearLine()
	return nil
}
