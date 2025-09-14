package store

import (
	"encoding/json"
	"fmt"
	"os"

	"github.com/abiosoft/colima/config"
	"github.com/sirupsen/logrus"
)

// Store stores internal Colima configuration for an instance
type Store struct {
	// if the runtime disk has been formatted.
	DiskFormatted bool `json:"disk_formatted"`
}

func storeFile() string { return config.CurrentProfile().StoreFile() }

// Load loads the store from the json file.
func Load() (s Store, err error) {
	b, err := os.ReadFile(storeFile())
	if err != nil {
		return s, fmt.Errorf("cannot read store file: %w", err)
	}

	if err := json.Unmarshal(b, &s); err != nil {
		return s, fmt.Errorf("error unmarshaling store file: %w", err)
	}

	return s, nil
}

// save persists the store.
func save(s Store) error {
	b, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return fmt.Errorf("error marshaling store: %w", err)
	}

	if err := os.WriteFile(storeFile(), b, 0o644); err != nil {
		return fmt.Errorf("error writing store file: %w", err)
	}

	return nil
}

// Set provides an easy way to set a value in the store.
func Set(f func(*Store)) error {
	s, err := Load()
	if err != nil {
		logrus.Debug("error loading store: %w", err)
	}

	f(&s)

	if err := save(s); err != nil {
		return fmt.Errorf("error saving store: %w", err)
	}

	return nil
}
