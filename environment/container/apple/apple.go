package apple

import (
	"context"
	"fmt"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/daemon/process/socktainer"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/util"
)

// Name is the container runtime name.
const Name = "apple"

// SocktainerCommand is the command for the socktainer Docker API bridge.
const SocktainerCommand = "socktainer"

var _ environment.Container = (*appleRuntime)(nil)

func init() {
	// Only register on supported systems (macOS 26+ on Apple Silicon)
	if util.MacOS26OrNewer() && environment.HostArch() == environment.AARCH64 {
		environment.RegisterContainer(Name, newRuntime, false)
	}
}

type appleRuntime struct {
	host  environment.HostActions
	guest environment.GuestActions
	cli.CommandChain

	pendingUpdates []componentUpdate // set by CheckUpdate, consumed by DownloadUpdate/InstallUpdate
}

// newRuntime creates a new Apple Container runtime.
func newRuntime(host environment.HostActions, guest environment.GuestActions) environment.Container {
	return &appleRuntime{
		host:         host,
		guest:        guest,
		CommandChain: cli.New(Name),
	}
}

// Name returns the name of the container runtime.
func (a appleRuntime) Name() string {
	return Name
}

// Provision provisions/installs the container runtime.
func (a appleRuntime) Provision(ctx context.Context) error {
	chain := a.Init(ctx)

	conf, _ := ctx.Value(config.CtxKey()).(config.Config)

	// Setup docker context pointing to socktainer socket
	// (socktainer is started by the daemon manager)
	chain.Add(a.setupContext)

	if conf.AutoActivate() {
		chain.Add(a.useContext)
	}

	return chain.Exec()
}

// Start starts the container runtime.
func (a appleRuntime) Start(ctx context.Context) error {
	// Socktainer is managed by the daemon, no action needed here
	// Docker context was already set up during Provision
	return nil
}

// Running returns if the container runtime is currently running.
func (a appleRuntime) Running(ctx context.Context) bool {
	// Check if socktainer socket exists and docker can connect
	return a.host.RunQuiet("docker", "--context", contextName(), "info") == nil
}

// Stop stops the container runtime.
func (a appleRuntime) Stop(ctx context.Context) error {
	// Socktainer is managed by the daemon, only clear docker context
	chain := a.Init(ctx)
	chain.Add(a.teardownContext)
	return chain.Exec()
}

// Teardown tears down/uninstalls the container runtime.
func (a appleRuntime) Teardown(ctx context.Context) error {
	// Socktainer is managed by the daemon, only clear docker context
	chain := a.Init(ctx)
	chain.Add(a.teardownContext)
	return chain.Exec()
}

// Update is not used for Apple runtime; updates use the AppUpdater interface instead.
func (a *appleRuntime) Update(ctx context.Context) (bool, error) {
	return false, nil
}

// Version returns the container runtime version.
func (a appleRuntime) Version(ctx context.Context) string {
	var parts []string

	if version, err := containerCurrentVersion(a.host); err == nil {
		parts = append(parts, fmt.Sprintf("Apple Container: %s", version))
	}

	if version, err := socktainerCurrentVersion(a.host); err == nil {
		parts = append(parts, fmt.Sprintf("Socktainer: %s", version))
	}

	return strings.Join(parts, "\n")
}

// HostSocketFile returns the path to the docker socket on host.
func HostSocketFile() string {
	return socktainer.SocketFile()
}

func contextName() string {
	return config.CurrentProfile().ID
}

func (a appleRuntime) contextCreated() bool {
	return a.host.RunQuiet("docker", "context", "inspect", contextName()) == nil
}

func (a appleRuntime) setupContext() error {
	if a.contextCreated() {
		return nil
	}

	return a.host.Run("docker", "context", "create", contextName(),
		"--description", "Colima Apple Container",
		"--docker", "host=unix://"+HostSocketFile(),
	)
}

func (a appleRuntime) useContext() error {
	return a.host.Run("docker", "context", "use", contextName())
}

func (a appleRuntime) teardownContext() error {
	if !a.contextCreated() {
		return nil
	}

	return a.host.Run("docker", "context", "rm", "--force", contextName())
}

