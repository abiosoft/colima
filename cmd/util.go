package cmd

import (
	"github.com/abiosoft/colima"
	"github.com/spf13/cobra"
)

func newApp() colima.App {
	app, err := colima.New()
	cobra.CheckErr(err)
	return app
}
