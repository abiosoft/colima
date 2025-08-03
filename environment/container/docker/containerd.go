package docker

import (
	"context"
	_ "embed"
	"fmt"
)

const containerdConfFile = "/etc/containerd/config.toml"
const containerdConfFileBackup = "/etc/containerd/config.colima.bak.toml"

//go:embed config.toml
var containerdConf []byte

func (d dockerRuntime) provisionContainerd(ctx context.Context) error {
	a := d.Init(ctx)

	// containerd config
	a.Add(func() error {
		if _, err := d.guest.Stat(containerdConfFileBackup); err == nil {
			// backup already exists, no need to overwrite
			return nil
		}

		// backup existing containerd config
		if err := d.guest.Run("sudo", "cp", containerdConfFile, containerdConfFileBackup); err != nil {
			return fmt.Errorf("error backing up %s: %w", containerdConfFile, err)
		}

		// write new containerd config
		if err := d.guest.Write(containerdConfFile, containerdConf); err != nil {
			return fmt.Errorf("error writing %s: %w", containerdConfFile, err)
		}

		return nil
	})

	a.Add(func() error {
		// restart containerd service
		return d.guest.Run("sudo", "service", "containerd", "restart")
	})

	return a.Exec()
}
