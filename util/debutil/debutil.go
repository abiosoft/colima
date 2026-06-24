package debutil

import (
	"context"
	"fmt"
	"strings"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/environment"
)

// packages is list of deb package names.
type packages []string

// Upgradable returns the shell command to check if the packages are upgradable with apt.
// The returned command should be passed to 'sh -c' or equivalent.
func (p packages) Upgradable() string {
	cmd := "sudo apt list --upgradable | grep"
	for _, v := range p {
		cmd += fmt.Sprintf(" -e '^%s/'", v)
	}
	return cmd
}

// Install returns the shell command to install the packages with apt.
// The returned command should be passed to 'sh -c' or equivalent.
func (p packages) Install() string {
	return "sudo apt-get install -y --allow-change-held-packages " + strings.Join(p, " ")
}

func UpdateRuntime(
	ctx context.Context,
	guest environment.GuestActions,
	chain cli.CommandChain,
	packageNames ...string,
) (bool, error) {
	a := chain.Init(ctx)
	log := a.Logger()

	packages := packages(packageNames)

	hasUpdates := false
	updated := false

	a.Stage("refreshing package manager")
	a.Add(func() error {
		return guest.RunQuiet(
			"sh",
			"-c",
			"sudo apt-get update -y",
		)
	})

	a.Stage("checking for updates")
	a.Add(func() error {
		err := guest.RunQuiet(
			"sh",
			"-c",
			packages.Upgradable(),
		)
		hasUpdates = err == nil
		return nil
	})

	a.Add(func() (err error) {
		if !hasUpdates {
			log.Warnln("no updates available")
			return
		}

		log.Println("updating packages ...")
		err = guest.RunQuiet(
			"sh",
			"-c",
			packages.Install(),
		)
		if err == nil {
			updated = true
			log.Println("done")
		}
		return
	})

	// it is necessary to execute the chain here to get the correct value for `updated`.
	err := a.Exec()
	return updated, err
}
