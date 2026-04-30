package model

import (
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/abiosoft/colima/environment/vm/native"
	"github.com/abiosoft/colima/util"
)

// newGuest creates the appropriate VM based on the current instance config.
// This centralizes VM creation that was previously hardcoded as lima.New()
// in 10+ places across the model package.
func newGuest() environment.VM {
	conf, _ := configmanager.LoadInstance()
	if conf.VMType == "native" && util.Linux() {
		return native.New(host.New())
	}
	return lima.New(host.New())
}
