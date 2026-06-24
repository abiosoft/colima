package lima

import (
	"context"
	"fmt"
	"path/filepath"

	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/util/downloader"
)

func (l limaVM) copyCerts() error {
	log := l.Logger(context.Background())
	err := func() error {
		dockerCertsDirHost := filepath.Join(docker.DockerDir(), "certs.d")
		dockerCertsDirsGuest := []string{"/etc/docker/certs.d", "/etc/ssl/certs"}
		if _, err := l.host.Stat(dockerCertsDirHost); err != nil {
			// no certs found
			return nil
		}

		// copy certs from host to a temp location in the guest using limactl copy,
		// then use sudo to move them to the final destinations.
		tmpDir := "/tmp/docker-certs"
		if err := l.RunQuiet("rm", "-rf", tmpDir); err != nil {
			return err
		}
		if err := l.RunQuiet("mkdir", "-p", tmpDir); err != nil {
			return err
		}
		if err := downloader.CopyToGuest(l.host, dockerCertsDirHost, tmpDir); err != nil {
			return err
		}

		// move from temp to final destinations
		for _, dir := range dockerCertsDirsGuest {
			if err := l.RunQuiet("sudo", "mkdir", "-p", dir); err != nil {
				return err
			}
			if err := l.RunQuiet("sudo", "cp", "-R", tmpDir+"/.", dir); err != nil {
				return err
			}
		}

		// cleanup temp
		_ = l.RunQuiet("rm", "-rf", tmpDir)

		return nil
	}()

	// not a fatal error, a warning suffices.
	if err != nil {
		log.Warnln(fmt.Errorf("cannot copy registry certs to vm: %w", err))
	}
	return nil
}
