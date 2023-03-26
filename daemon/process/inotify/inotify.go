package inotify

import (
	"context"
	"fmt"
	"time"

	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/sirupsen/logrus"
)

const Name = "inotify"

func CtxKeyGuest() any { return struct{ name string }{name: "inotify_guest"} }
func CtxKeyDirs() any  { return struct{ name string }{name: "inotify_dirs"} }

// New returns inotify process.
func New() process.Process {
	return &inotifyProcess{
		log: logrus.WithField("context", "inotify"),
	}
}

var _ process.Process = (*inotifyProcess)(nil)

type inotifyProcess struct {
	vmVols []string
	guest  environment.GuestActions

	log *logrus.Entry
}

// Alive implements process.Process
func (f *inotifyProcess) Alive(ctx context.Context) error {
	daemonRunning, _ := ctx.Value(process.CtxKeyDaemon()).(bool)

	// if the parent is active, we can assume inotify is active.
	if daemonRunning {
		return nil
	}
	return fmt.Errorf("inotify not running")
}

// Dependencies implements process.Process
func (*inotifyProcess) Dependencies() (deps []process.Dependency, root bool) {
	return nil, false
}

// Name implements process.Process
func (*inotifyProcess) Name() string {
	return Name
}

// Start implements process.Process
func (f *inotifyProcess) Start(ctx context.Context) error {
	guest, ok := ctx.Value(CtxKeyGuest()).(environment.GuestActions)
	if !ok {
		return fmt.Errorf("environment.GuestActions missing in context")
	}
	f.vmVols, ok = ctx.Value(CtxKeyDirs()).([]string)
	if !ok {
		return fmt.Errorf("dirs missing in context")
	}
	f.vmVols = omitChildrenDirectories(f.vmVols)

	f.guest = guest
	log := f.log

	log.Info("waiting for VM to start")
	f.waitForLima(ctx)
	log.Info("VM started")

	watcher := &defaultWatcher{log: log}

	return f.handleEvents(ctx, watcher)
}

// waitForLima waits until lima starts and sets the directory to watch.
func (f *inotifyProcess) waitForLima(ctx context.Context) {
	log := f.log

	// wait for Lima to finish starting
	for {
		log.Info("waiting 5 secs for VM")

		// 5 second interval
		after := time.After(time.Second * 5)

		select {
		case <-ctx.Done():
			return
		case <-after:
			i, err := limautil.Instance()
			if err != nil || !i.Running() {
				continue
			}
			if err := f.guest.RunQuiet("uname", "-a"); err == nil {
				return
			}
		}
	}
}
