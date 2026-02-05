package apple

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/apple"
	"github.com/abiosoft/colima/util/downloader"
	"github.com/coreos/go-semver/semver"
)

// GitHub repository path for version checking.
const socktainerRepo = "socktainer/socktainer"

// componentUpdate describes an available update for socktainer.
type componentUpdate struct {
	Name           string // display name
	CurrentVersion string // currently installed version
	LatestVersion  string // latest version from GitHub
	PkgURL         string // download URL for the .pkg file
}

// Ensure appleRuntime implements AppUpdater.
var _ environment.AppUpdater = (*appleRuntime)(nil)

// CheckUpdate checks for available updates for socktainer.
func (a *appleRuntime) CheckUpdate(ctx context.Context) (environment.UpdateInfo, error) {
	log := a.Logger(ctx)
	log.Println("checking for updates ...")

	update, err := checkSocktainerUpdate(a.host)
	if err != nil {
		return environment.UpdateInfo{}, err
	}

	if update == nil {
		a.pendingUpdates = nil
		return environment.UpdateInfo{}, nil
	}

	a.pendingUpdates = []componentUpdate{*update}

	return environment.UpdateInfo{
		Available:   true,
		Description: fmt.Sprintf("  %s: %s -> %s\n", update.Name, update.CurrentVersion, update.LatestVersion),
	}, nil
}

// DownloadUpdate downloads the update package.
// Called before the instance is stopped.
func (a *appleRuntime) DownloadUpdate(ctx context.Context) error {
	log := a.Logger(ctx)

	for _, u := range a.pendingUpdates {
		// Clear cached download to ensure the latest version is fetched,
		// since the /releases/latest/ URL does not change between versions.
		_ = os.Remove(downloader.CacheFilename(u.PkgURL))

		log.Println("downloading", u.Name, u.LatestVersion, "...")
		if _, err := downloader.Download(a.host, downloader.Request{URL: u.PkgURL}); err != nil {
			return fmt.Errorf("failed to download %s %s: %w", u.Name, u.LatestVersion, err)
		}
	}

	return nil
}

// InstallUpdate installs the previously downloaded update package.
// Called after the instance is stopped.
func (a *appleRuntime) InstallUpdate(ctx context.Context) error {
	log := a.Logger(ctx)

	for _, u := range a.pendingUpdates {
		// Download returns from cache since already downloaded in DownloadUpdate.
		pkgFile, err := downloader.Download(a.host, downloader.Request{URL: u.PkgURL})
		if err != nil {
			return fmt.Errorf("failed to get cached package for %s: %w", u.Name, err)
		}

		// Copy to a temporary location with .pkg extension for installer.
		tmpPkg := filepath.Join(os.TempDir(), u.Name+".pkg")
		if err := a.host.RunQuiet("cp", pkgFile, tmpPkg); err != nil {
			return fmt.Errorf("failed to prepare package for %s: %w", u.Name, err)
		}
		defer func() { _ = os.Remove(tmpPkg) }()

		log.Println("installing", u.Name, u.LatestVersion, "(sudo password may be required) ...")
		if err := a.host.RunInteractive("sudo", "installer", "-pkg", tmpPkg, "-target", "/"); err != nil {
			return fmt.Errorf("failed to install %s: %w", u.Name, err)
		}
	}

	a.pendingUpdates = nil
	return nil
}

// checkSocktainerUpdate checks if socktainer has an available update.
// Returns nil if already up to date.
func checkSocktainerUpdate(host environment.HostActions) (*componentUpdate, error) {
	current, err := socktainerCurrentVersion(host)
	if err != nil {
		return nil, fmt.Errorf("error getting socktainer version: %w", err)
	}

	latest, err := latestGitHubVersion(host, socktainerRepo)
	if err != nil {
		return nil, fmt.Errorf("error checking latest socktainer version: %w", err)
	}

	if !isNewer(latest, current) {
		return nil, nil
	}

	return &componentUpdate{
		Name:           SocktainerCommand,
		CurrentVersion: current,
		LatestVersion:  latest,
		PkgURL:         socktainerPkgURL,
	}, nil
}

// containerCurrentVersion returns the installed version of the container CLI.
// Output format: [{"version":"0.8.0","appName":"container",...}]
func containerCurrentVersion(host environment.HostActions) (string, error) {
	output, err := host.RunOutput(apple.ContainerCommand, "system", "version", "--format=json")
	if err != nil {
		return "", fmt.Errorf("error running container system version: %w", err)
	}

	var versions []struct {
		Version string `json:"version"`
	}
	if err := json.Unmarshal([]byte(strings.TrimSpace(output)), &versions); err != nil {
		return "", fmt.Errorf("error parsing container version JSON: %w", err)
	}
	if len(versions) == 0 {
		return "", fmt.Errorf("empty container version output")
	}

	return versions[0].Version, nil
}

// socktainerCurrentVersion returns the installed version of socktainer.
// Output format: socktainer: v0.9.0 (git commit: 6a710bf)
func socktainerCurrentVersion(host environment.HostActions) (string, error) {
	output, err := host.RunOutput(SocktainerCommand, "--version")
	if err != nil {
		return "", fmt.Errorf("error running socktainer --version: %w", err)
	}

	// Extract version from "socktainer: v0.9.0 (git commit: ...)"
	parts := strings.Fields(strings.TrimSpace(output))
	for _, p := range parts {
		if strings.HasPrefix(p, "v") {
			return p, nil
		}
	}

	return "", fmt.Errorf("could not parse socktainer version from: %s", output)
}

// latestGitHubVersion resolves the latest release version tag from a GitHub
// repository by following the /releases/latest redirect.
func latestGitHubVersion(host environment.HostActions, repo string) (string, error) {
	latestURL := fmt.Sprintf("https://github.com/%s/releases/latest", repo)

	// Follow redirects to get the final URL.
	// e.g. https://github.com/socktainer/socktainer/releases/latest
	//   -> https://github.com/socktainer/socktainer/releases/tag/v1.2.3
	finalURL, err := host.RunOutput("curl", "-ILs", "-o", "/dev/null", "-w", "%{url_effective}", latestURL)
	if err != nil {
		return "", fmt.Errorf("error resolving latest release: %w", err)
	}

	// Extract version tag from the URL path.
	tag := path.Base(strings.TrimSpace(finalURL))
	if tag == "" || tag == "." || tag == "latest" {
		return "", fmt.Errorf("could not determine latest version from: %s", finalURL)
	}

	return tag, nil
}

// isNewer returns true if latest is a newer semver than current.
func isNewer(latest, current string) bool {
	latestVer, err := parseSemver(latest)
	if err != nil {
		return false
	}
	currentVer, err := parseSemver(current)
	if err != nil {
		return false
	}
	return latestVer.Compare(*currentVer) > 0
}

// parseSemver parses a version string into a semver, stripping a leading "v"
// and padding to three components if needed (e.g. "1.2" -> "1.2.0").
func parseSemver(version string) (*semver.Version, error) {
	v := strings.TrimPrefix(version, "v")
	for strings.Count(v, ".") < 2 {
		v += ".0"
	}
	return semver.NewVersion(v)
}