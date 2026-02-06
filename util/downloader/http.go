package downloader

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util/terminal"
	"github.com/schollz/progressbar/v3"
	"golang.org/x/term"
)

// HTTPClient encapsulates HTTP download operations
type HTTPClient struct {
	client    *http.Client
	userAgent string
}

// DownloadOptions configures a download operation
type DownloadOptions struct {
	URL            string
	DestPath       string
	ExpectedETag   string // for resume validation
	ResumeFromByte int64  // byte offset to resume from
	ShowProgress   bool
}

// DownloadResult contains metadata about the completed download
type DownloadResult struct {
	FinalURL   string // After following redirects
	ETag       string // For future resume validation
	TotalBytes int64
	WasResumed bool
}

// ResumeInfo stores metadata for resumable downloads
type ResumeInfo struct {
	ETag         string `json:"etag"`
	BytesWritten int64  `json:"bytes_written"`
}

// NewHTTPClient creates a configured HTTP client
func NewHTTPClient() *HTTPClient {
	transport := &http.Transport{
		// use proxy from environment (HTTP_PROXY, HTTPS_PROXY, NO_PROXY)
		Proxy: http.ProxyFromEnvironment,
		DialContext: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).DialContext,
		TLSHandshakeTimeout:   10 * time.Second,
		ResponseHeaderTimeout: 30 * time.Second,
		ExpectContinueTimeout: 1 * time.Second,
	}

	return &HTTPClient{
		client: &http.Client{
			Transport: transport,
			// checkRedirect is left default - Go follows up to 10 redirects
			// and returns the final response
		},
		userAgent: "colima/" + config.AppVersion().Version,
	}
}

// GetFinalURL follows redirects and returns the final URL
func (h *HTTPClient) GetFinalURL(ctx context.Context, rawURL string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodHead, rawURL, nil)
	if err != nil {
		return "", fmt.Errorf("invalid URL '%s': %w", rawURL, err)
	}
	req.Header.Set("User-Agent", h.userAgent)

	resp, err := h.client.Do(req)
	if err != nil {
		return "", &NetworkError{Op: "resolve redirect", URL: rawURL, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	// check for HTTP errors
	if resp.StatusCode >= 400 {
		return "", &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        rawURL,
		}
	}

	// resp.Request.URL contains the final URL after redirects
	return resp.Request.URL.String(), nil
}

// Download performs a file download with optional resume support
func (h *HTTPClient) Download(ctx context.Context, opts DownloadOptions) (*DownloadResult, error) {
	result := &DownloadResult{}

	// open destination file for writing (or appending if resuming)
	var file *os.File
	var existingSize int64
	var err error

	if opts.ResumeFromByte > 0 {
		file, err = os.OpenFile(opts.DestPath, os.O_WRONLY|os.O_APPEND, 0644)
		if err != nil {
			// can't resume, start fresh
			opts.ResumeFromByte = 0
			opts.ExpectedETag = ""
		} else {
			existingSize = opts.ResumeFromByte
		}
	}

	if file == nil {
		file, err = os.Create(opts.DestPath)
		if err != nil {
			return nil, fmt.Errorf("cannot create file '%s': %w", opts.DestPath, err)
		}
	}
	defer func() { _ = file.Close() }()

	// build request
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, opts.URL, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL '%s': %w", opts.URL, err)
	}
	req.Header.Set("User-Agent", h.userAgent)

	// add Range header for resume
	if existingSize > 0 {
		req.Header.Set("Range", fmt.Sprintf("bytes=%d-", existingSize))
		// add If-Range with ETag if available for safe resume
		if opts.ExpectedETag != "" {
			req.Header.Set("If-Range", opts.ExpectedETag)
		}
	}

	// execute request
	resp, err := h.client.Do(req)
	if err != nil {
		return nil, &NetworkError{Op: "download", URL: opts.URL, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	// store final URL after redirects
	result.FinalURL = resp.Request.URL.String()
	result.ETag = resp.Header.Get("ETag")

	// handle response status
	switch resp.StatusCode {
	case http.StatusOK: // 200 - Full content (resume not supported or If-Range failed)
		if existingSize > 0 {
			// server sent full content, need to truncate and start over
			if err := file.Truncate(0); err != nil {
				return nil, fmt.Errorf("cannot truncate file for fresh download: %w", err)
			}
			if _, err := file.Seek(0, 0); err != nil {
				return nil, fmt.Errorf("cannot seek to start of file: %w", err)
			}
			existingSize = 0
		}
		result.TotalBytes = resp.ContentLength

	case http.StatusPartialContent: // 206 - Resume successful
		result.WasResumed = true
		// Content-Range: bytes 21010-47021/47022
		contentRange := resp.Header.Get("Content-Range")
		if totalSize := parseContentRangeTotal(contentRange); totalSize > 0 {
			result.TotalBytes = totalSize
		} else {
			result.TotalBytes = existingSize + resp.ContentLength
		}

	case http.StatusRequestedRangeNotSatisfiable: // 416
		// file is likely complete or server doesn't support range
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        opts.URL,
		}

	default:
		if resp.StatusCode >= 400 {
			return nil, &HTTPStatusError{
				StatusCode: resp.StatusCode,
				Status:     resp.Status,
				URL:        opts.URL,
			}
		}
	}

	// set up progress bar
	var writer io.Writer = file
	var bar *progressbar.ProgressBar
	var progressWriter *terminal.ProgressWriter
	if opts.ShowProgress && isTerminal() {
		// get output from context for coordination
		if output := terminal.OutputFromContext(ctx); output != nil {
			progressWriter = output.ProgressWriter()
			progressWriter.Begin()
		}
		bar = h.createProgressBar(result.TotalBytes, existingSize)
		writer = io.MultiWriter(file, bar)
	}

	// stream response body to file
	written, err := io.Copy(writer, resp.Body)
	if err != nil {
		if progressWriter != nil {
			progressWriter.End()
		}
		return result, &NetworkError{Op: "download", URL: opts.URL, Err: err}
	}

	// finish progress bar
	if bar != nil {
		_ = bar.Finish()
	}

	// resume status output
	if progressWriter != nil {
		progressWriter.End()
	}

	result.TotalBytes = existingSize + written
	return result, nil
}

// Fetch downloads content from a URL and returns it as bytes (for small files like SHA checksums)
func (h *HTTPClient) Fetch(ctx context.Context, url string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid URL '%s': %w", url, err)
	}
	req.Header.Set("User-Agent", h.userAgent)

	resp, err := h.client.Do(req)
	if err != nil {
		return nil, &NetworkError{Op: "fetch", URL: url, Err: err}
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		return nil, &HTTPStatusError{
			StatusCode: resp.StatusCode,
			Status:     resp.Status,
			URL:        url,
		}
	}

	// limit read to 1MB for safety (SHA files should be tiny)
	return io.ReadAll(io.LimitReader(resp.Body, 1<<20))
}

// createProgressBar creates a progress bar for download visualization
func (h *HTTPClient) createProgressBar(totalBytes, startOffset int64) *progressbar.ProgressBar {
	opts := []progressbar.Option{
		progressbar.OptionSetDescription("    "),
		progressbar.OptionSetWriter(os.Stderr),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionThrottle(100 * time.Millisecond),
		progressbar.OptionClearOnFinish(),
		progressbar.OptionSetPredictTime(true),
		progressbar.OptionSetRenderBlankState(true),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "=",
			SaucerHead:    ">",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	}

	// if total size is unknown, use a spinner
	if totalBytes <= 0 {
		opts = append(opts, progressbar.OptionSpinnerType(11))
		totalBytes = -1
	}

	bar := progressbar.NewOptions64(totalBytes, opts...)

	// if resuming, set initial progress
	if startOffset > 0 {
		_ = bar.Set64(startOffset)
	}

	return bar
}

// parseContentRangeTotal extracts total size from Content-Range header
// Format: "bytes 21010-47021/47022" or "bytes 21010-47021/*"
func parseContentRangeTotal(header string) int64 {
	if header == "" {
		return -1
	}
	parts := strings.Split(header, "/")
	if len(parts) != 2 || parts[1] == "*" {
		return -1
	}
	total, err := strconv.ParseInt(parts[1], 10, 64)
	if err != nil {
		return -1
	}
	return total
}

// isTerminal returns true if stderr is a terminal
func isTerminal() bool {
	return term.IsTerminal(int(os.Stderr.Fd()))
}
