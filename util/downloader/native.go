package downloader

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"time"
)

// nativeDownloader uses Go's native HTTP client
type nativeDownloader struct{}

// Download downloads a file using Go's native HTTP client
func (n *nativeDownloader) Download(r Request, destPath string) error {
	d := downloader{}
	// check for existing partial download and resume info
	var resumeInfo ResumeInfo
	resumeInfoPath := d.resumeInfoPath(r.URL)
	if data, err := os.ReadFile(resumeInfoPath); err == nil {
		_ = json.Unmarshal(data, &resumeInfo)
	}

	// get existing file size for resume
	var existingSize int64
	if stat, err := os.Stat(destPath); err == nil {
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
		DestPath:       destPath,
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

	return nil
}
