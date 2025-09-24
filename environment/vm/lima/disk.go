package lima

import (
	"context"
	"fmt"
	"os"
	"strings"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/incus"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/store"
	"github.com/abiosoft/colima/util/downloader"
	"gopkg.in/yaml.v3"
)

func (l *limaVM) createRuntimeDisk(conf config.Config) error {
	if environment.IsNoneRuntime(conf.Runtime) {
		// runtime disk is not required when no runtime is in use
		return nil
	}

	disk := dataDisk(conf.Runtime)

	s, _ := store.Load()
	format := !s.DiskFormatted // only format if not previously formatted

	if !limautil.HasDisk() {
		if err := limautil.CreateDisk(conf.Disk); err != nil {
			return fmt.Errorf("error creating runtime disk: %w", err)
		}
		format = true // new disk should be formated
	}

	// when disk is formatted for the wrong runtime, prevent use
	if s.DiskFormatted && s.DiskRuntime != "" && s.DiskRuntime != conf.Runtime {
		return fmt.Errorf("runtime disk provisioned for %s runtime. Delete container data with 'colima delete --data' before using another runtime", s.DiskRuntime)
	}

	l.limaConf.Disk = config.Disk(conf.RootDisk).GiB()
	l.limaConf.AdditionalDisks = append(l.limaConf.AdditionalDisks, limaconfig.Disk{
		Name:   config.CurrentProfile().ID,
		Format: format,
		FSType: disk.FSType,
	})

	l.mountRuntimeDisk(conf)
	return nil
}

func (l *limaVM) useRuntimeDisk(conf config.Config) {
	if !limautil.HasDisk() {
		l.limaConf.Disk = config.Disk(conf.Disk).GiB()
		return
	}

	disk := dataDisk(conf.Runtime)

	s, _ := store.Load()
	format := !s.DiskFormatted // only format if not previously formatted

	l.limaConf.Disk = config.Disk(conf.RootDisk).GiB()
	l.limaConf.AdditionalDisks = append(l.limaConf.AdditionalDisks, limaconfig.Disk{
		Name:   config.CurrentProfile().ID,
		Format: format,
		FSType: disk.FSType,
	})

	l.mountRuntimeDisk(conf)
}

func dataDisk(runtime string) environment.DataDisk {
	switch runtime {
	case docker.Name:
		return docker.DataDisk()
	case containerd.Name:
		return containerd.DataDisk()
	case incus.Name:
		return incus.DataDisk()
	}

	return environment.DataDisk{}
}

func (l *limaVM) mountRuntimeDisk(conf config.Config) {
	disk := dataDisk(conf.Runtime)

	// pre mount script
	for _, script := range disk.PreMount {
		l.limaConf.Provision = append(l.limaConf.Provision, limaconfig.Provision{
			Mode:   "dependency",
			Script: script,
		})
	}

	mountPoint := limautil.MountPoint()
	for _, dir := range disk.Dirs {
		script := strings.NewReplacer(
			"{mount_point}", mountPoint,
			"{name}", dir.Name,
			"{data_path}", dir.Path,
		).Replace("mkdir -p {mount_point}/{name} {data_path} && mount --bind {mount_point}/{name} {data_path}")

		l.limaConf.Provision = append(l.limaConf.Provision, limaconfig.Provision{
			Mode:   "dependency",
			Script: script,
		})
	}
}

func (l *limaVM) downloadDiskImage(ctx context.Context, conf config.Config) error {
	log := l.Logger(ctx)

	// use a user specified disk image
	if conf.DiskImage != "" {
		if _, err := os.Stat(conf.DiskImage); err != nil {
			return fmt.Errorf("invalid disk image: %w", err)
		}

		image, err := limautil.Image(l.limaConf.Arch, conf.Runtime)
		if err != nil {
			return fmt.Errorf("error getting disk image details: %w", err)
		}

		sha := downloader.SHA{Size: 512, Digest: image.Digest}
		if err := sha.ValidateFile(l.host, conf.DiskImage); err != nil {
			return fmt.Errorf("disk image must be downloaded from '%s', hash failure: %w", image.Location, err)
		}

		image.Location = conf.DiskImage
		l.limaConf.Images = []limaconfig.File{image}
		return nil
	}

	// use a previously cached image
	if image, ok := limautil.ImageCached(l.limaConf.Arch, conf.Runtime); ok {
		l.limaConf.Images = []limaconfig.File{image}
		return nil
	}

	// download image
	log.Infoln("downloading disk image ...")
	image, err := limautil.DownloadImage(l.limaConf.Arch, conf.Runtime)
	if err != nil {
		return fmt.Errorf("error getting qcow image: %w", err)
	}

	l.limaConf.Images = []limaconfig.File{image}
	return nil
}

func (l *limaVM) setDiskImage() error {
	var c limaconfig.Config
	b, err := os.ReadFile(config.CurrentProfile().LimaFile())
	if err != nil {
		return err
	}
	if err := yaml.Unmarshal(b, &c); err != nil {
		return err
	}

	l.limaConf.Images = c.Images
	return nil
}

func (l *limaVM) syncDiskSize(ctx context.Context, conf config.Config) config.Config {
	log := l.Logger(ctx)
	instance, err := configmanager.LoadInstance()
	if err != nil {
		// instance config missing, ignore
		return conf
	}

	resized := func() bool {
		if instance.Disk == conf.Disk {
			// nothing to do
			return false
		}

		size := conf.Disk - instance.Disk
		if size < 0 {
			log.Warnln("disk size cannot be reduced, ignoring...")
			return false
		}

		if err := limautil.ResizeDisk(conf.Disk); err != nil {
			log.Warnln(fmt.Errorf("unable to resize disk: %w", err))
			return false
		}

		log.Printf("resizing disk to %dGiB...", conf.Disk)
		return true
	}()

	if !resized {
		conf.Disk = instance.Disk
	}

	return conf
}
