package docker

import (
	"context"
	"os"
	"path/filepath"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/guest/systemctl"
	"github.com/abiosoft/colima/util"
	"github.com/abiosoft/colima/util/debutil"
)

// Name is container runtime name.
const Name = "docker"

var _ environment.Container = (*dockerRuntime)(nil)

func init() {
	environment.RegisterContainer(Name, newRuntime, false)
}

type dockerRuntime struct {
	host      environment.HostActions
	guest     environment.GuestActions
	systemctl systemctl.Systemctl
	cli.CommandChain
}

// newRuntime creates a new docker runtime.
func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &dockerRuntime{
		host:         host,
		guest:        guest,
		systemctl:    systemctl.New(guest),
		CommandChain: cli.New(Name),
	}
}

func (d dockerRuntime) Name() string {
	return Name
}

func (d dockerRuntime) Provision(ctx context.Context) error {
	a := d.Init(ctx)
	log := d.Logger(ctx)

	conf, _ := ctx.Value(config.CtxKey()).(config.Config)

	// provision containerd
	a.Add(func() error {
		return d.provisionContainerd(ctx)
	})

	// daemon.json
	a.Add(func() error {
		// these are not fatal errors
		if err := d.createDaemonFile(conf.Docker, conf.Env); err != nil {
			log.Warnln(err)
		}
		if err := d.addHostGateway(conf.Docker); err != nil {
			log.Warnln(err)
		}
		if err := d.reloadAndRestartSystemdService(); err != nil {
			log.Warnln(err)
		}
		return nil
	})

	// docker context
	a.Add(d.setupContext)
	if conf.AutoActivate() {
		a.Add(d.useContext)
	}

	return a.Exec()
}

func (d dockerRuntime) Start(ctx context.Context) error {
	a := d.Init(ctx)

	a.Retry("", time.Second, 60, func(int) error {
		return d.systemctl.Start("docker.service")
	})

	// service startup takes few seconds, retry for a minute before giving up.
	a.Retry("", time.Second, 60, func(int) error {
		return d.guest.RunQuiet("sudo", "docker", "info")
	})

	// ensure docker is accessible without root
	// otherwise, restart to ensure user is added to docker group
	a.Add(func() error {
		if err := d.guest.RunQuiet("docker", "info"); err == nil {
			return nil
		}
		ctx := context.WithValue(ctx, cli.CtxKeyQuiet, true)
		return d.guest.Restart(ctx)
	})

	return a.Exec()
}

func (d dockerRuntime) Running(ctx context.Context) bool {
	return d.systemctl.Active("docker.service")
}

func (d dockerRuntime) Stop(ctx context.Context, force bool) error {
	a := d.Init(ctx)

	a.Add(func() error {
		if !d.Running(ctx) {
			return nil
		}
		return d.systemctl.Stop("docker.service", force)
	})

	// clear docker context settings
	// since the container runtime can be changed on startup,
	// it is better to not leave unnecessary traces behind
	a.Add(d.teardownContext)

	return a.Exec()
}

func (d dockerRuntime) Teardown(ctx context.Context) error {
	a := d.Init(ctx)

	// clear docker context settings
	a.Add(d.teardownContext)

	return a.Exec()
}

func (d dockerRuntime) Dependencies() []string {
	return []string{"docker"}
}

func (d dockerRuntime) Version(ctx context.Context) string {
	version, _ := d.host.RunOutput("docker", "--context", config.CurrentProfile().ID, "version", "--format", `client: v{{.Client.Version}}{{printf "\n"}}server: v{{.Server.Version}}`)
	return version
}

func (d *dockerRuntime) Update(ctx context.Context) (bool, error) {
	packages := []string{
		"docker-ce",
		"docker-ce-cli",
		"containerd.io",
	}

	return debutil.UpdateRuntime(ctx, d.guest, d, packages...)
}

// DataDirs represents the data disk for the container runtime.
func DataDisk() environment.DataDisk {
	return environment.DataDisk{
		Dirs:   diskDirs,
		FSType: "ext4",
		PreMount: []string{
			"systemctl stop docker.service",
			"systemctl stop containerd.service",
		},
	}
}

var diskDirs = []environment.DiskDir{
	{Name: "docker", Path: "/var/lib/docker"},
	{Name: "containerd", Path: "/var/lib/containerd"},
	{Name: "rancher", Path: "/var/lib/rancher"},
	{Name: "cni", Path: "/var/lib/cni"},
	{Name: "ramalama", Path: "/var/lib/ramalama"},
}

// DockerDir returns the path to Docker config.
func DockerDir() string {
	// if DOCKER_CONFIG env var is set, obey it.
	if dir := os.Getenv("DOCKER_CONFIG"); dir != "" {
		return dir
	}

	return filepath.Join(util.HomeDir(), ".docker")
}
