package util

import (
	"fmt"
	"github.com/sirupsen/logrus"
	"log"
	"os"
)

func HomeDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		// this should never happen
		log.Fatal(fmt.Errorf("error retrieving home directory: %w", err))
	}
	return home
}

var logger *logrus.Logger

// Logger returns the global logger instance.
func Logger() *logrus.Logger {
	if logger == nil {
		logger = logrus.New()
	}
	return logger
}
