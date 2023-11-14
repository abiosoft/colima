package lima

import (
	"context"
	"io"

	"github.com/abiosoft/colima/config"
)

func (l limaVM) Run(args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() error {
		return l.host.Run(args...)
	})

	return a.Exec()
}

func (l limaVM) SSH(workingDir string, args ...string) error {
	args = append([]string{limactl, "shell", "--workdir", workingDir, config.CurrentProfile().ID}, args...)

	a := l.Init(context.Background())

	a.Add(func() error {
		return l.host.RunInteractive(args...)
	})

	return a.Exec()
}

func (l limaVM) RunInteractive(args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() error {
		return l.host.RunInteractive(args...)
	})

	return a.Exec()
}

func (l limaVM) RunWith(stdin io.Reader, stdout io.Writer, args ...string) error {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() error {
		return l.host.RunWith(stdin, stdout, args...)
	})

	return a.Exec()
}

func (l limaVM) RunOutput(args ...string) (out string, err error) {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() (err error) {
		out, err = l.host.RunOutput(args...)
		return
	})

	err = a.Exec()
	return
}

func (l limaVM) RunQuiet(args ...string) (err error) {
	args = append([]string{lima}, args...)

	a := l.Init(context.Background())

	a.Add(func() (err error) {
		return l.host.RunQuiet(args...)
	})

	err = a.Exec()
	return
}
