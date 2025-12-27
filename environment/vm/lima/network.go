package lima

import (
	"fmt"
	"net"
	"os"
	"path/filepath"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
	"gopkg.in/yaml.v3"
)

var defaultLimaNetworkConfig = limautil.LimaNetwork{
	Networks: struct {
		UserV2 limautil.LimaNetworkConfig `yaml:"user-v2"`
	}{
		UserV2: limautil.LimaNetworkConfig{

			Mode:    "user-v2",
			Gateway: net.ParseIP("192.168.5.2"),
			Netmask: "255.255.255.0",
		},
	},
}

func (l *limaVM) writeNetworkFile(conf config.Config) error {
	networkFile := limautil.NetworkFile()

	// use custom gateway address
	gatewayAddress := conf.Network.GatewayAddress
	if gatewayAddress != nil {
		defaultLimaNetworkConfig.Networks.UserV2.Gateway = gatewayAddress
	}

	// if there are no running instances, clear network directory
	if instances, err := limautil.RunningInstances(); err == nil && len(instances) == 0 {
		if err := os.RemoveAll(limautil.NetworkAssetsDirectory()); err != nil {
			logrus.Warnln(fmt.Errorf("could not clear network assets directory: %w", err))
		}
	}

	if err := os.MkdirAll(filepath.Dir(networkFile), 0755); err != nil {
		return fmt.Errorf("error creating Lima config directory: %w", err)
	}

	networkFileMarshalled, err := yaml.Marshal(&defaultLimaNetworkConfig)
	if err != nil {
		return fmt.Errorf("error marshalling Lima network config file: %w", err)
	}

	if err := os.WriteFile(networkFile, networkFileMarshalled, 0755); err != nil {
		return fmt.Errorf("error writing Lima network config file: %w", err)
	}
	return nil
}

func (l *limaVM) replicateHostAddresses(conf config.Config) error {
	if !conf.Network.Address && conf.Network.HostAddresses {
		for _, ip := range util.HostIPAddresses() {
			if err := l.RunQuiet("sudo", "ip", "address", "add", ip.String()+"/24", "dev", "lo"); err != nil {
				return err
			}
		}
	}
	return nil
}

func (l *limaVM) removeHostAddresses() {
	conf, _ := configmanager.LoadInstance()
	if !conf.Network.Address && conf.Network.HostAddresses {
		for _, ip := range util.HostIPAddresses() {
			_ = l.RunQuiet("sudo", "ip", "address", "del", ip.String()+"/24", "dev", "lo")
		}
	}
}
