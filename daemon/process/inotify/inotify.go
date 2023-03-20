package inotify

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
)

const Name = "inotify"

const watchInterval = time.Second * 1
const volumesInterval = time.Second * 5

var CtxKeyGuest = struct{ name string }{name: "inotify_guest"}

// New returns inotify process.
func New() process.Process {
	return &inotifyProcess{
		interval: watchInterval,
		log:      logrus.WithField("context", "inotify"),
	}
}

var _ process.Process = (*inotifyProcess)(nil)

type inotifyProcess struct {
	containerVols []string
	// will only be used for containerVols
	sync.RWMutex

	interval time.Duration
	vmVols   []string
	guest    environment.GuestActions
	runtime  string

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
	guest, ok := ctx.Value(CtxKeyGuest).(environment.GuestActions)
	if !ok {
		return fmt.Errorf("environment.GuestActions missing in context")
	}

	f.guest = guest
	log := f.log

	log.Info("waiting for VM to start")
	f.waitForLima(ctx)
	log.Info("VM started")

	c, err := limautil.InstanceConfig()
	if err != nil {
		return fmt.Errorf("error retrieving config")
	}
	f.runtime = c.Runtime

	for _, mount := range c.MountsOrDefault() {
		p, err := util.CleanPath(mount.Location)
		if err != nil {
			return fmt.Errorf("error retrieving mount path: %w", err)
		}
		f.vmVols = append(f.vmVols, p)
	}

	return f.watch(ctx)
}

// waitForLima waits until lima starts and sets the directory to watch.
func (f *inotifyProcess) waitForLima(ctx context.Context) {
	log := f.log

	// wait for Lima to finish starting
	for {
		log.Info("attempting to fetch config from Lima")

		// 5 second interval
		after := time.After(time.Second * 5)

		select {
		case <-ctx.Done():
			return
		case <-after:
			i, err := limautil.Instance()
			if err == nil && i.Running() {
				if _, err := limautil.InstanceConfig(); err == nil {
					return
				}
			}
		}
	}
}

func (f *inotifyProcess) watch(ctx context.Context) error {
	if err := f.fetchContainerVolumes(ctx); err != nil {
		return fmt.Errorf("error fetching container volumes: %w", err)
	}

	return f.watchFiles(ctx)
}
