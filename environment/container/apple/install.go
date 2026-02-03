package apple

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment/vm/apple"
	"github.com/abiosoft/colima/util/downloader"
	log "github.com/sirupsen/logrus"
)

// GitHub release URLs for Apple Container dependencies
const (
	// containerPkgURL is the URL for the Apple Container CLI .pkg file.
	containerPkgURL = "https://github.com/apple/container/releases/latest/download/container-installer-signed.pkg"

	// socktainerPkgURL is the URL for the socktainer .pkg file.
	socktainerPkgURL = "https://github.com/socktainer/socktainer/releases/latest/download/socktainer-installer.pkg"
)

// isContainerInstalled checks if the Apple Container CLI is installed.
func isContainerInstalled() bool {
	_, err := exec.LookPath(apple.ContainerCommand)
	return err == nil
}

// isSocktainerInstalled checks if socktainer is installed.
func isSocktainerInstalled() bool {
	_, err := exec.LookPath(SocktainerCommand)
	return err == nil
}

// ensureDependencies checks for required dependencies and installs them if missing.
// Returns an error if the user declines installation or if installation fails.
func (a appleRuntime) ensureDependencies(logger *log.Entry) error {
	var missingDeps []string

	if !isContainerInstalled() {
		missingDeps = append(missingDeps, apple.ContainerCommand)
	}

	if !isSocktainerInstalled() {
		missingDeps = append(missingDeps, SocktainerCommand)
	}

	if len(missingDeps) == 0 {
		return nil
	}

	// Prompt user for installation
	logger.Println("the following dependencies are required but not installed:")
	for _, dep := range missingDeps {
		logger.Println("  - ", dep)
	}

	if !cli.Prompt("would you like to download and install them") {
		return fmt.Errorf("required dependencies not installed: %v", missingDeps)
	}

	// Install missing dependencies
	for _, dep := range missingDeps {
		if err := a.installDependency(logger, dep); err != nil {
			return fmt.Errorf("failed to install %s: %w", dep, err)
		}
	}

	return nil
}

// installDependency downloads and installs a dependency package.
func (a appleRuntime) installDependency(logger *log.Entry, dep string) error {
	var pkgURL string

	switch dep {
	case apple.ContainerCommand:
		pkgURL = containerPkgURL
	case SocktainerCommand:
		pkgURL = socktainerPkgURL
	default:
		return fmt.Errorf("unknown dependency: %s", dep)
	}

	logger.Println("downloading ", dep, "...")

	// Download the package
	pkgFile, err := downloader.Download(a.host, downloader.Request{URL: pkgURL})
	if err != nil {
		return fmt.Errorf("failed to download %s: %w", dep, err)
	}

	// Copy to a temporary location with .pkg extension for installer
	tmpPkg := filepath.Join(os.TempDir(), dep+".pkg")
	if err := a.host.RunQuiet("cp", pkgFile, tmpPkg); err != nil {
		return fmt.Errorf("failed to prepare package: %w", err)
	}
	defer func() {
		_ = os.Remove(tmpPkg)
	}()

	logger.Println("installing ", dep, "(requires sudo)...")

	// Install using macOS installer
	if err := a.host.RunInteractive("sudo", "installer", "-pkg", tmpPkg, "-target", "/"); err != nil {
		return fmt.Errorf("failed to install %s: %w", dep, err)
	}

	logger.Println(dep, " installed successfully")
	return nil
}

// Dependencies returns the dependencies required for Apple Container runtime.
// Only docker is required as an external dependency.
// The container CLI and socktainer are installed during provisioning if missing.
func (a appleRuntime) Dependencies() []string {
	return []string{"docker"}
}
