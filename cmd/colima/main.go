package main

import (
	"os"
	"path/filepath"

	_ "github.com/abiosoft/colima/cmd"      // for other commands
	_ "github.com/abiosoft/colima/embedded" // for embedded assets

	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/cmd/vmnet"
)

func main() {
	_, cmd := filepath.Split(os.Args[0])
	switch cmd {
	case "colima-vmnet":
		vmnet.Execute()
	default:
		root.Execute()
	}
}
