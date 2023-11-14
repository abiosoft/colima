package lima

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util/fsutil"
	"github.com/abiosoft/colima/util/terminal"
	"github.com/sirupsen/logrus"
)

var dependencyPackages = []string{
	// docker
	"docker.io",
	// utilities
	"htop", "vim", "inetutils-ping", "dnsutils",
}

// cacheDependencies downloads the ubuntu deb files to a path on the host.
// The return value is the directory of the downloaded deb files.
func (l *limaVM) cacheDependencies(log *logrus.Entry, conf config.Config) (string, error) {
	codename, err := l.RunOutput("sh", "-c", `grep "^UBUNTU_CODENAME" /etc/os-release | cut -d= -f2`)
	if err != nil {
		return "", fmt.Errorf("error retrieving OS version from vm: %w", err)
	}

	arch := environment.Arch(conf.Arch).Value()
	dir := filepath.Join(config.CacheDir(), "packages", codename, string(arch))
	if err := fsutil.MkdirAll(dir, 0755); err != nil {
		return "", fmt.Errorf("error creating cache directory for OS packages: %w", err)
	}

	doneFile := filepath.Join(dir, ".downloaded")
	if _, err := os.Stat(doneFile); err == nil {
		// already downloaded
		return dir, nil
	}

	output := ""
	for _, p := range dependencyPackages {
		line := fmt.Sprintf(`sudo apt-get install --reinstall --print-uris -qq "%s" | cut -d"'" -f2`, p)
		out, err := l.RunOutput("sh", "-c", line)
		if err != nil {
			return "", fmt.Errorf("error fetching dependencies list: %w", err)
		}
		output += out + " "
	}

	debPackages := strings.Fields(output)

	// progress bar for Ubuntu deb packages download.
	// TODO: extract this into re-usable progress bar for multi-downloads
	for i, p := range debPackages {
		// status feedback
		log.Infof("downloading package %d of %d ...", i+1, len(debPackages))

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
	// cache dependencies
	dir, err := l.cacheDependencies(log, conf)
	if err != nil {
		log.Warnln(fmt.Errorf("error caching dependencies: %w", err))
		log.Warnln("falling back to normal package install")
		return l.Run("sh", "-c", "sudo apt install -y "+strings.Join(dependencyPackages, " "))
	}

	// validate if packages were previously installed
	installed := true
	for _, p := range dependencyPackages {
		if err := l.RunQuiet("dpkg", "-s", p); err != nil {
			installed = false
			break
		}
	}

	if installed {
		return nil
	}

	// install packages
	return l.Run("sh", "-c", "sudo dpkg -i "+dir+"/*.deb")
}
