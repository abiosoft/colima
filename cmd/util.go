package cmd

import (
	"github.com/abiosoft/colima/app"
	"github.com/sirupsen/logrus"
)

func newApp() app.App {
	colimaApp, err := app.New()
	if err != nil {
		logrus.Fatal("Error: ", err)
	}
	return colimaApp
}
