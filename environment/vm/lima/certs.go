package lima

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util"
)

func (l limaVM) copyCerts() error {
	log := l.Logger(context.Background())
	err := func() error {
		dockerCertsDirHost := filepath.Join(util.HomeDir(), ".docker", "certs.d")
		dockerCertsDirsGuest := []string{"/etc/docker/certs.d", "/etc/ssl/certs"}
		if _, err := l.host.Stat(dockerCertsDirHost); err != nil {
			// no certs found
			return nil
		}

		// we are utilising the host cache path as it is the only guaranteed mounted path.

		// copy to cache dir
		dockerCertsCacheDir := filepath.Join(config.CacheDir(), "docker-certs")
		if err := l.host.RunQuiet("rm", "-rf", dockerCertsCacheDir); err != nil {
			return err
		}
		if err := l.host.RunQuiet("mkdir", "-p", dockerCertsCacheDir); err != nil {
			return err
		}
		if err := l.host.RunQuiet("cp", "-R", dockerCertsDirHost+"/.", dockerCertsCacheDir); err != nil {
			return err
		}

		// copy from cache to vm
		for _, dir := range dockerCertsDirsGuest {
			// copy from cache to vm
			if err := l.RunQuiet("sudo", "mkdir", "-p", dir); err != nil {
				return err
			}
			if err := l.RunQuiet("sudo", "cp", "-R", dockerCertsCacheDir+"/.", dir); err != nil {
				return err
			}
		}

		return nil
	}()

	// not a fatal error, a warning suffices.
	if err != nil {
		log.Warnln(fmt.Errorf("cannot copy registry certs to vm: %w", err))
	}
	return nil
}
