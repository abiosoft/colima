package config

import (
	"path/filepath"
	"strings"
)

var profile = &Profile{ID: AppName, DisplayName: AppName, ShortName: "default"}

// SetProfile sets the profile name for the application.
// This is an avenue to test Colima without breaking an existing stable setup.
// Not perfect, but good enough for testing.
func SetProfile(profileName string) {
	profile = ProfileFromName(profileName)
}

// ProfileFromName retrieves profile given name.
func ProfileFromName(name string) *Profile {
	var i Profile

	switch name {
	case "", AppName, "default":
		i.ID = AppName
		i.DisplayName = AppName
		i.ShortName = "default"
		return &i
	}

	// sanitize
	name = strings.TrimPrefix(name, "colima-")

	// if custom profile is specified,
	// use a prefix to prevent possible name clashes
	i.ID = "colima-" + name
	i.DisplayName = "colima [profile=" + name + "]"
	i.ShortName = name
	return &i
}

// CurrentProfile returns the current running profile.
func CurrentProfile() *Profile { return profile }

// Profile is colima profile.
type Profile struct {
	ID          string
	DisplayName string
	ShortName   string

	configDir *requiredDir
}

// ConfigDir returns the configuration directory.
func (p *Profile) ConfigDir() string {
	if p.configDir == nil {
		p.configDir = &requiredDir{
			dir: func() (string, error) {
				return filepath.Join(configBaseDir.Dir(), p.ShortName), nil
			},
		}
	}
	return p.configDir.Dir()
}

// LimaInstanceDir returns the directory for the Lima instance.
func (p *Profile) LimaInstanceDir() string {
	return filepath.Join(limaDir.Dir(), p.ID)
}

// File returns the path to the config file.
func (p *Profile) File() string {
	return filepath.Join(p.ConfigDir(), configFileName)
}

// LimaFile returns the path to the lima config file.
func (p *Profile) LimaFile() string {
	return filepath.Join(p.LimaInstanceDir(), "lima.yaml")
}

// StateFile returns the path to the state file.
func (p *Profile) StateFile() string {
	return filepath.Join(p.LimaInstanceDir(), configFileName)
}

var _ ProfileInfo = (*Profile)(nil)

// ProfileInfo is the information about a profile.
type ProfileInfo interface {
	// ConfigDir returns the configuration directory.
	ConfigDir() string

	// LimaInstanceDir returns the directory for the Lima instance.
	LimaInstanceDir() string

	// File returns the path to the config file.
	File() string

	// LimaFile returns the path to the lima config file.
	LimaFile() string

	// StateFile returns the path to the state file.
	StateFile() string
}
