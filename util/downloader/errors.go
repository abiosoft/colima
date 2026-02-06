package downloader

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"path"
	"syscall"
)

// Sentinel errors for type checking
var (
	ErrNetworkConnection = errors.New("network connection error")
	ErrHTTPStatus        = errors.New("HTTP error")
	ErrResumeFailed      = errors.New("resume failed")
	ErrSHAValidation     = errors.New("SHA validation failed")
)

// NetworkError wraps network-related errors with user-friendly messages
type NetworkError struct {
	Op  string // "connect", "resolve", "download"
	URL string
	Err error
}

func (e *NetworkError) Error() string {
	return fmt.Sprintf("%s failed for '%s': %s", e.Op, e.URL, e.friendlyMessage())
}

func (e *NetworkError) Unwrap() error {
	return e.Err
}

func (e *NetworkError) friendlyMessage() string {
	// check for DNS resolution errors
	var dnsErr *net.DNSError
	if errors.As(e.Err, &dnsErr) {
		return fmt.Sprintf("DNS lookup failed for host '%s'. Check your network connection or DNS settings", dnsErr.Name)
	}

	// check for connection refused
	var opErr *net.OpError
	if errors.As(e.Err, &opErr) {
		if errors.Is(opErr.Err, syscall.ECONNREFUSED) {
			return "connection refused. The server may be down or unreachable"
		}
		if opErr.Timeout() {
			return "connection timed out. Check your network connection or try again later"
		}
	}

	// check for URL parsing errors
	var urlErr *url.Error
	if errors.As(e.Err, &urlErr) {
		if urlErr.Timeout() {
			return "request timed out. The server may be slow or overloaded"
		}
	}

	return e.Err.Error()
}

// HTTPStatusError represents HTTP error responses
type HTTPStatusError struct {
	StatusCode int
	Status     string
	URL        string
}

func (e *HTTPStatusError) Error() string {
	switch e.StatusCode {
	case 404:
		return fmt.Sprintf("file not found at '%s'. The URL may be incorrect or the file may have been removed", e.URL)
	case 403:
		return fmt.Sprintf("access forbidden to '%s'. You may need authentication or the resource is restricted", e.URL)
	case 401:
		return fmt.Sprintf("authentication required for '%s'", e.URL)
	case 500, 502, 503, 504:
		return fmt.Sprintf("server error (%d) at '%s'. Try again later", e.StatusCode, e.URL)
	case 416: // Range Not Satisfiable
		return fmt.Sprintf("resume failed for '%s'. The server does not support the requested byte range", e.URL)
	default:
		return fmt.Sprintf("HTTP %d (%s) for '%s'", e.StatusCode, e.Status, e.URL)
	}
}

func (e *HTTPStatusError) Unwrap() error {
	return ErrHTTPStatus
}

// ResumeError indicates a failed resume attempt
type ResumeError struct {
	Reason string
	URL    string
}

func (e *ResumeError) Error() string {
	return fmt.Sprintf("cannot resume download of '%s': %s. Starting fresh download", path.Base(e.URL), e.Reason)
}

func (e *ResumeError) Unwrap() error {
	return ErrResumeFailed
}

// SHAValidationError indicates checksum mismatch
type SHAValidationError struct {
	File     string
	Expected string
	Actual   string
	Size     int // 256 or 512
}

func (e *SHAValidationError) Error() string {
	return fmt.Sprintf("SHA%d checksum mismatch for '%s':\n  expected: %s\n  actual:   %s\nThe file may be corrupted or tampered with. Delete the cached file and retry",
		e.Size, e.File, e.Expected, e.Actual)
}

func (e *SHAValidationError) Unwrap() error {
	return ErrSHAValidation
}
