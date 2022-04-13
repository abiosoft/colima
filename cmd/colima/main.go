package main

import (
	"os"
	"path/filepath"

	_ "github.com/abiosoft/colima/cmd"       // for other commands
	_ "github.com/abiosoft/colima/cmd/vmnet" // for vmnet daemon command
	_ "github.com/abiosoft/colima/embedded"  // for embedded assets

	"github.com/abiosoft/colima/cmd/root"
)

func main() {
	_, cmd := filepath.Split(os.Args[0])
	switch cmd {
	default:
		root.Execute()
	}
}
