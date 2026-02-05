package apple

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/downloader"
	log "github.com/sirupsen/logrus"
)

// SocktainerPkgURL is the URL for the socktainer .pkg file.
const SocktainerPkgURL = "https://github.com/socktainer/socktainer/releases/latest/download/socktainer-installer.pkg"

// SocktainerCommand is the command for the socktainer Docker API bridge.
const SocktainerCommand = "socktainer"

// IsSocktainerInstalled checks if socktainer is installed.
func IsSocktainerInstalled() bool {
	_, err := exec.LookPath(SocktainerCommand)
	return err == nil
}

// ensureSocktainer checks if socktainer is installed and installs it if missing.
// Returns an error if the user declines installation or if installation fails.
func ensureSocktainer(host environment.HostActions, logger *log.Entry) error {
	if IsSocktainerInstalled() {
		return nil
	}

	// Prompt user for installation
	logger.Println("socktainer is required but not installed")

	if !cli.Prompt("would you like to download and install it (sudo password may be required)") {
		return fmt.Errorf("socktainer is required but not installed")
	}

	return InstallSocktainer(host, logger)
}

// InstallSocktainer downloads and installs socktainer.
func InstallSocktainer(host environment.HostActions, logger *log.Entry) error {
	logger.Println("downloading socktainer ...")

	// Download the package
	pkgFile, err := downloader.Download(host, downloader.Request{URL: SocktainerPkgURL})
	if err != nil {
		return fmt.Errorf("failed to download socktainer: %w", err)
	}

	// Copy to a temporary location with .pkg extension for installer
	tmpPkg := filepath.Join(os.TempDir(), "socktainer.pkg")
	if err := host.RunQuiet("cp", pkgFile, tmpPkg); err != nil {
		return fmt.Errorf("failed to prepare package: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpPkg)
	}()

	logger.Println("installing socktainer (sudo password may be required) ...")

	// Install using macOS installer
	if err := host.RunInteractive("sudo", "installer", "-pkg", tmpPkg, "-target", "/"); err != nil {
		return fmt.Errorf("failed to install socktainer: %w", err)
	}

	logger.Println("socktainer installed successfully")
	return nil
}
