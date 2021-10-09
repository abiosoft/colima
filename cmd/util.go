package cmd

import (
	"github.com/abiosoft/colima"
	"github.com/sirupsen/logrus"
)

func newApp() colima.App {
	app, err := colima.New()
	if err != nil {
		logrus.Fatal("Error: ", err)
	}
	return app
}
