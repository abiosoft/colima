package main

import (
	_ "github.com/abiosoft/colima/cmd" // for other commands
	"github.com/abiosoft/colima/cmd/root"
)

func main() {
	root.Execute()
}
