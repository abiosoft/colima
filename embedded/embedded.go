package embedded

import (
	"embed"
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/sirupsen/logrus"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

// only include binaries suffixed with .bin.
//go:embed binaries/*.bin
var binaries embed.FS

func init() {
	if err := os.MkdirAll(extractDirectory(), 0755); err != nil {
		logrus.Fatal(fmt.Errorf("cannot create extract directory"))
	}
	if err := extractAllBinaries(); err != nil {
		logrus.Fatal(err)
	}
}

func extractAllBinaries() error {
	dirs, err := binaries.ReadDir("binaries")
	if err != nil {
		return fmt.Errorf("could not read embedded binaries: %w", err)
	}

	for _, dir := range dirs {
		err := extractBinary(dir.Name())
		if err != nil {
			return err
		}
	}

	return nil
}

func extractBinary(fileName string) error {
	src, err := binaries.Open("binaries/" + fileName)
	if err != nil {
		return fmt.Errorf("could not open embedded binary '%v': %w", fileName, err)
	}
	defer func() { _ = src.Close() }()

	dstPath := binaryFile(fileName).destPath()
	dst, err := os.OpenFile(dstPath, os.O_CREATE|os.O_RDWR|os.O_TRUNC, 0755)
	if err != nil {
		return fmt.Errorf("cannot extract embedded binary: %w", err)
	}
	defer func() { _ = dst.Close() }()

	if _, err := io.Copy(dst, src); err != nil {
		return fmt.Errorf("cannot extract embedded binary: %w", err)
	}
	return nil
}

func extractDirectory() string {
	return filepath.Join(config.CacheDir(), "binaries", config.AppVersion().Version)
}

type binaryFile string

func (b binaryFile) destPath() string {
	return filepath.Join(extractDirectory(), strings.TrimSuffix(string(b), ".bin"))
}

// File returns the filepath for the embedded binary.
func File(fileName string) (string, error) {
	_, err := fs.Stat(binaries, "binaries/"+fileName)
	if err != nil {
		return "", fmt.Errorf("error retrieving embedded binary: %w", err)
	}
	return binaryFile(fileName).destPath(), nil
}
