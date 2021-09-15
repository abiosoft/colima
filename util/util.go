package util

import (
	"fmt"
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

func User() string {
	user := os.Getenv("USER")
	if user == "" {
		log.Fatal("could not retrieve OS user")
	}
	return user
}
