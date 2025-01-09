package downloader

import (
	"fmt"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/abiosoft/colima/util"
)

// SHA is the shasum of a file.
type SHA struct {
	Digest string // shasum
	URL    string // url to download the shasum file (if Digest is empty)
	Size   int    // one of 256 or 512
}

// ValidateFile validates the SHA of the file.
func (s SHA) ValidateFile(host hostActions, file string) error {
	dir, filename := filepath.Split(file)
	digest := strings.TrimPrefix(s.Digest, fmt.Sprintf("sha%d:", s.Size))
	shasumBinary := "shasum"
	if util.MacOS() {
		shasumBinary = "/usr/bin/shasum"
	}

	script := strings.NewReplacer(
		"{dir}", dir,
		"{digest}", digest,
		"{size}", strconv.Itoa(s.Size),
		"{filename}", filename,
		"{shasum_bin}", shasumBinary,
	).Replace(
		`cd {dir} && echo "{digest}  {filename}" | {shasum_bin} -a {size} --check --status`,
	)

	return host.Run("sh", "-c", script)
}

func (s SHA) validateDownload(host hostActions, url string, filename string) error {
	if s.URL == "" && s.Digest == "" {
		return fmt.Errorf("error validating SHA: one of Digest or URL must be set")
	}

	// fetch digest from URL if empty
	if s.Digest == "" {
		// retrieve the filename from the download url.
		filename := func() string {
			if url == "" {
				return ""
			}
			split := strings.Split(url, "/")
			return split[len(split)-1]
		}()

		digest, err := fetchSHAFromURL(host, s.URL, filename)
		if err != nil {
			return err
		}
		s.Digest = digest
	}

	return s.ValidateFile(host, filename)
}

func fetchSHAFromURL(host hostActions, url, filename string) (string, error) {
	script := strings.NewReplacer(
		"{url}", url,
		"{filename}", filename,
	).Replace(
		"curl -sL {url} | grep '  {filename}$' | awk -F' ' '{print $1}'",
	)
	sha, err := host.RunOutput("sh", "-c", script)
	if err != nil {
		return "", fmt.Errorf("error retrieving sha from url '%s': %w", url, err)
	}
	return strings.TrimSpace(sha), nil
}
