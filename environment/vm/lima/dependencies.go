package lima

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/deb"
	"github.com/abiosoft/colima/util/fsutil"
	"github.com/abiosoft/colima/util/terminal"
	"github.com/sirupsen/logrus"
)

// cacheDependencies downloads the ubuntu deb files to a path on the host.
// The return value is the directory of the downloaded deb files.
func (l *limaVM) cacheDependencies(src deb.URISource, log *logrus.Entry, conf config.Config) (string, error) {
	codename, err := l.RunOutput("sh", "-c", `grep "^UBUNTU_CODENAME" /etc/os-release | cut -d= -f2`)
	if err != nil {
		return "", fmt.Errorf("error retrieving OS version from vm: %w", err)
	}

	arch := environment.Arch(conf.Arch).Value()
	dir := filepath.Join(config.CacheDir(), "packages", codename, string(arch), src.Name())
	if err := fsutil.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("error creating cache directory for OS packages: %w", err)
	}

	doneFile := filepath.Join(dir, ".downloaded")
	if _, err := os.Stat(doneFile); err == nil {
		// already downloaded
		return dir, nil
	}

	var debPackages []string
	packages, err := src.URIs(arch)
	if err != nil {
		return "", fmt.Errorf("error fetching package URIs using %s: %w", src.Name(), err)
	}
	debPackages = append(debPackages, packages...)

	// progress bar for Ubuntu deb packages download.
	// TODO: extract this into re-usable progress bar for multi-downloads
	for i, p := range debPackages {
		// status feedback
		logrus.Infof("downloading package %d of %d ...", i+1, len(debPackages))

		// download
		if err := l.host.RunInteractive(
			"sh", "-c",
			fmt.Sprintf(`cd %s && curl -LO -# %s`, dir, p),
		); err != nil {
			return "", fmt.Errorf("error downloading dependency: %w", err)
		}

		// clear terminal
		terminal.ClearLine() // for curl output
		terminal.ClearLine() // for log message
	}

	// write a file to signify it is done
	return dir, l.host.RunQuiet("touch", doneFile)
}

func (l *limaVM) installDependencies(log *logrus.Entry, conf config.Config) error {
	srcs := []deb.URISource{
		&deb.Mantic{Guest: l},
		&deb.Docker{Host: l.host, Guest: l},
	}

	for _, src := range srcs {
		if src.Installed() {
			// already installed
			continue
		}

		if err := src.PreInstall(); err != nil {
			log.Warn(fmt.Errorf("preinstall check failed for %s: %w", src.Name(), err))
		}

		// cache dependencies
		dir, err := l.cacheDependencies(src, log, conf)
		if err != nil {
			log.Warnln(fmt.Errorf("error caching dependencies for %s: %w", src.Name(), err))
			log.Warnln("falling back to normal package install")

			if err := src.Install(); err != nil {
				return fmt.Errorf("error installing packages using %s: %w", src.Name(), err)
			}

			// installed
			continue
		}

		// validate if packages were previously installed
		installed := true
		for _, p := range src.Packages() {
			if err := l.RunQuiet("dpkg", "-s", p); err != nil {
				installed = false
				break
			}
		}

		if installed {
			continue
		}

		// install packages
		if err := l.Run("sh", "-c", "sudo dpkg -i "+dir+"/*.deb"); err != nil {
			log.Warn(fmt.Errorf("error installing packages using %s: %w", src.Name(), err))
			log.Warnln("falling back to normal package install")

			if err := src.Install(); err != nil {
				return fmt.Errorf("error installing packages using %s: %w", src.Name(), err)
			}
		}

	}

	return nil
}
