package colima

import (
	"github.com/abiosoft/colima/runtime/container"
	"github.com/abiosoft/colima/runtime/host"
	"github.com/abiosoft/colima/runtime/vm"
)

type App interface {
	Start() error
	Stop() error
	// TODO consider making a struct
}

var _ App = (*colimaApp)(nil)

type colimaApp struct {
	host       host.Runtime
	guest      vm.Runtime
	containers map[string]container.Runtime
}

func (c colimaApp) Start() error {
	return nil
}

func (c colimaApp) Stop() error {
	return nil
}
