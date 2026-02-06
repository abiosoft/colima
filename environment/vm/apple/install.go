package apple

import (
	"archive/zip"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/downloader"
	log "github.com/sirupsen/logrus"
)

// SocktainerZipURL is the URL for the socktainer zip file.
const SocktainerZipURL = "https://github.com/socktainer/socktainer/releases/latest/download/socktainer.zip"

// SocktainerCommand is the command name for the socktainer Docker API bridge.
const SocktainerCommand = "socktainer"

// socktainerDir is the directory name for socktainer installation.
const socktainerDir = "_socktainer"

// socktainerBinDir returns the directory where socktainer binary is installed.
func socktainerBinDir() string {
	return filepath.Join(config.Dir(), socktainerDir, "bin")
}

// SocktainerBinPath returns the absolute path to the socktainer binary.
func SocktainerBinPath() string {
	return filepath.Join(socktainerBinDir(), SocktainerCommand)
}

// socktainerInstalled checks if socktainer is installed.
func socktainerInstalled() bool {
	_, err := os.Stat(SocktainerBinPath())
	return err == nil
}

// ensureSocktainer checks if socktainer is installed and installs it if missing.
func ensureSocktainer(host environment.HostActions, logger *log.Entry) error {
	if socktainerInstalled() {
		return nil
	}

	return InstallSocktainer(host, logger)
}

// InstallSocktainer downloads and installs socktainer.
func InstallSocktainer(host environment.HostActions, logger *log.Entry) error {
	logger.Println("downloading socktainer ...")

	// Download the zip file
	zipFile, err := downloader.Download(host, downloader.Request{URL: SocktainerZipURL})
	if err != nil {
		return fmt.Errorf("failed to download socktainer: %w", err)
	}

	// Create the bin directory
	binDir := socktainerBinDir()
	if err := os.MkdirAll(binDir, 0755); err != nil {
		return fmt.Errorf("failed to create socktainer bin directory: %w", err)
	}

	logger.Println("extracting socktainer ...")

	// Extract the zip file
	if err := extractZip(zipFile, binDir); err != nil {
		return fmt.Errorf("failed to extract socktainer: %w", err)
	}

	// Make the binary executable
	binPath := SocktainerBinPath()
	if err := os.Chmod(binPath, 0755); err != nil {
		return fmt.Errorf("failed to make socktainer executable: %w", err)
	}

	logger.Println("socktainer installed successfully")
	return nil
}

// extractZip extracts a zip file to the destination directory.
func extractZip(zipPath, destDir string) error {
	r, err := zip.OpenReader(zipPath)
	if err != nil {
		return err
	}
	defer r.Close()

	for _, f := range r.File {
		// Only extract files, skip directories
		if f.FileInfo().IsDir() {
			continue
		}

		// Extract to destination directory (flatten structure)
		destPath := filepath.Join(destDir, filepath.Base(f.Name))

		outFile, err := os.OpenFile(destPath, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, f.Mode())
		if err != nil {
			return err
		}

		rc, err := f.Open()
		if err != nil {
			outFile.Close()
			return err
		}

		_, err = io.Copy(outFile, rc)
		rc.Close()
		outFile.Close()

		if err != nil {
			return err
		}
	}

	return nil
}