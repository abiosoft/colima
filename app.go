package colima

import (
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/runtime"
	"github.com/abiosoft/colima/runtime/container"
	"github.com/abiosoft/colima/runtime/container/docker"
	"github.com/abiosoft/colima/runtime/host"
	"github.com/abiosoft/colima/runtime/vm"
)

type App interface {
	Start() error
	Stop() error
}

var _ App = (*colimaApp)(nil)

func New(c config.Config) (App, error) {
	vmConfig := vm.Config{
		CPU:     c.VM.CPU,
		Disk:    c.VM.Disk,
		Memory:  c.VM.Memory,
		SSHPort: config.SSHPort(),
		Changed: false,
	}

	guest := vm.New(vmConfig)
	if err := host.IsInstalled(guest); err != nil {
		return nil, fmt.Errorf("dependency check failed for VM: %w", err)
	}

	controller := runtime.ControllerFrom(guest.Host(), guest)
	dockerRuntime := docker.New(controller)
	if err := host.IsInstalled(dockerRuntime); err != nil {
		return nil, fmt.Errorf("dependency check failed for docker: %w", err)
	}

	return &colimaApp{}, nil
}

type colimaApp struct {
	host       host.Runtime
	guest      vm.Runtime
	containers []container.Runtime
}

func (c colimaApp) Start() error {
	return nil
}

func (c colimaApp) Stop() error {
	return nil
}
