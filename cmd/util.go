package cmd

import (
	"github.com/abiosoft/colima/app"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/sirupsen/logrus"
)

func newApp() app.App {
	colimaApp, err := app.New(root.RootCmdArgs.Verbose)
	if err != nil {
		logrus.Fatal("Error: ", err)
	}
	return colimaApp
}
