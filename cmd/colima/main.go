package main

import (
	_ "github.com/abiosoft/colima/cmd" // for other commands
	"github.com/abiosoft/colima/cmd/root"
	_ "github.com/abiosoft/colima/embedded" // for testing
)

func main() {
	root.Execute()
}
