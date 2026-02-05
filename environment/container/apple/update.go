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

// GitHub repository paths for version checking.
const (
	containerRepo  = "apple/container"
	socktainerRepo = "socktainer/socktainer"
)

// componentUpdate describes an available update for a single component.
type componentUpdate struct {
	Name           string // display name (e.g. "container")
	CurrentVersion string // currently installed version
	LatestVersion  string // latest version from GitHub
	PkgURL         string // download URL for the .pkg file (from install.go constants)
}

// Ensure appleRuntime implements AppUpdater.
var _ environment.AppUpdater = (*appleRuntime)(nil)

// CheckUpdate checks for available updates for container and socktainer.
func (a *appleRuntime) CheckUpdate(ctx context.Context) (environment.UpdateInfo, error) {
	log := a.Logger(ctx)
	log.Println("checking for updates ...")

	updates, err := checkUpdates(a.host)
	if err != nil {
		return environment.UpdateInfo{}, err
	}

	a.pendingUpdates = updates

	if len(updates) == 0 {
		return environment.UpdateInfo{}, nil
	}

	var desc strings.Builder
	for _, u := range updates {
		fmt.Fprintf(&desc, "  %s: %s -> %s\n", u.Name, u.CurrentVersion, u.LatestVersion)
	}

	return environment.UpdateInfo{
		Available:   true,
		Description: desc.String(),
	}, nil
}

// DownloadUpdate downloads the update packages.
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

// InstallUpdate installs the previously downloaded update packages.
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

		// container requires stopping the system and uninstalling before updating.
		if u.Name == apple.ContainerCommand {
			log.Println("uninstalling previous", u.Name, "version ...")
			_ = a.host.RunQuiet(apple.ContainerCommand, "system", "stop")
			if err := a.host.RunInteractive("/usr/local/bin/uninstall-container.sh", "-k"); err != nil {
				return fmt.Errorf("failed to uninstall previous %s version: %w", u.Name, err)
			}
		}

		log.Println("installing", u.Name, u.LatestVersion, "(sudo password may be required) ...")
		if err := a.host.RunInteractive("sudo", "installer", "-pkg", tmpPkg, "-target", "/"); err != nil {
			return fmt.Errorf("failed to install %s: %w", u.Name, err)
		}
	}

	a.pendingUpdates = nil
	return nil
}

// checkUpdates checks both container and socktainer for available updates.
// Returns only the components that have updates available.
func checkUpdates(host environment.HostActions) ([]componentUpdate, error) {
	type component struct {
		name   string
		repo   string
		pkgURL string
		// getVersion returns the currently installed version for this component.
		getVersion func() (string, error)
	}

	components := []component{
		{
			name:       apple.ContainerCommand,
			repo:       containerRepo,
			pkgURL:     containerPkgURL,
			getVersion: func() (string, error) { return containerCurrentVersion(host) },
		},
		{
			name:       SocktainerCommand,
			repo:       socktainerRepo,
			pkgURL:     socktainerPkgURL,
			getVersion: func() (string, error) { return socktainerCurrentVersion(host) },
		},
	}

	var updates []componentUpdate

	for _, comp := range components {
		current, err := comp.getVersion()
		if err != nil {
			return nil, fmt.Errorf("error getting %s version: %w", comp.name, err)
		}

		latest, err := latestGitHubVersion(host, comp.repo)
		if err != nil {
			return nil, fmt.Errorf("error checking latest %s version: %w", comp.name, err)
		}

		if isNewer(latest, current) {
			updates = append(updates, componentUpdate{
				Name:           comp.name,
				CurrentVersion: current,
				LatestVersion:  latest,
				PkgURL:         comp.pkgURL,
			})
		}
	}

	return updates, nil
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
	// e.g. https://github.com/apple/container/releases/latest
	//   -> https://github.com/apple/container/releases/tag/v1.2.3
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
