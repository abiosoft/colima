package main

import (
	_ "github.com/abiosoft/colima/cmd"        // for other commands
	_ "github.com/abiosoft/colima/cmd/daemon" // for vmnet daemon
	_ "github.com/abiosoft/colima/embedded"   // for embedded assets

	"github.com/abiosoft/colima/cmd/root"
)

func main() {
	root.Execute()
}
