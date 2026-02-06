package downloader

import (
	"bufio"
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/sha512"
	"fmt"
	"hash"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SHA is the shasum of a file.
type SHA struct {
	Digest string // shasum
	URL    string // url to download the shasum file (if Digest is empty)
	Size   int    // one of 256 or 512
}

// ValidateFile validates the SHA of the file.
// The host parameter is kept for API compatibility but is not used.
func (s SHA) ValidateFile(host hostActions, file string) error {
	return s.validateFile(file)
}

// validateFile performs SHA validation using pure Go crypto.
func (s SHA) validateFile(file string) error {
	// open the file
	f, err := os.Open(file)
	if err != nil {
		return fmt.Errorf("cannot open file for validation: %w", err)
	}
	defer func() { _ = f.Close() }()

	// select hash algorithm
	var h hash.Hash
	switch s.Size {
	case 256:
		h = sha256.New()
	case 512:
		h = sha512.New()
	default:
		return fmt.Errorf("unsupported SHA size: %d (must be 256 or 512)", s.Size)
	}

	// compute hash
	if _, err := io.Copy(h, f); err != nil {
		return fmt.Errorf("error reading file for SHA validation: %w", err)
	}

	// compare
	computed := fmt.Sprintf("%x", h.Sum(nil))
	expected := strings.TrimPrefix(s.Digest, fmt.Sprintf("sha%d:", s.Size))
	expected = strings.ToLower(strings.TrimSpace(expected))

	if computed != expected {
		return &SHAValidationError{
			File:     filepath.Base(file),
			Expected: expected,
			Actual:   computed,
			Size:     s.Size,
		}
	}

	return nil
}

func (s SHA) validateDownload(url string, filename string) error {
	if s.URL == "" && s.Digest == "" {
		return fmt.Errorf("error validating SHA: one of Digest or URL must be set")
	}

	// fetch digest from URL if empty
	if s.Digest == "" {
		// retrieve the filename from the download url.
		targetFilename := ""
		if url != "" {
			split := strings.Split(url, "/")
			targetFilename = split[len(split)-1]
		}

		digest, err := fetchSHAFromURL(s.URL, targetFilename)
		if err != nil {
			return err
		}
		s.Digest = digest
	}

	return s.validateFile(filename)
}

// fetchSHAFromURL fetches SHA checksum file and extracts digest for the target file
func fetchSHAFromURL(shaURL, targetFilename string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	client := NewHTTPClient()

	// fetch SHA file content
	data, err := client.Fetch(ctx, shaURL)
	if err != nil {
		return "", fmt.Errorf("error downloading SHA file from '%s': %w", shaURL, err)
	}

	// parse SHA file to find the matching entry
	digest, err := parseSHAContent(data, targetFilename)
	if err != nil {
		return "", fmt.Errorf("error parsing SHA file from '%s': %w", shaURL, err)
	}

	return digest, nil
}

// parseSHAContent reads SHA checksum content and extracts the digest for the target filename.
// Supports formats:
//   - GNU coreutils: "<hash>  <filename>" (two spaces)
//   - BSD/binary mode: "<hash> *<filename>" (space + asterisk)
func parseSHAContent(data []byte, targetFilename string) (string, error) {
	scanner := bufio.NewScanner(bytes.NewReader(data))
	for scanner.Scan() {
		line := scanner.Text()
		// format: "<hash>  <filename>" (two spaces) or "<hash> *<filename>" (binary mode)
		parts := strings.Fields(line)
		if len(parts) >= 2 {
			hash := parts[0]
			filename := strings.TrimPrefix(parts[len(parts)-1], "*")

			if filename == targetFilename || strings.HasSuffix(filename, "/"+targetFilename) {
				return hash, nil
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return "", err
	}

	return "", fmt.Errorf("no SHA entry found for '%s' in checksum file", targetFilename)
}
