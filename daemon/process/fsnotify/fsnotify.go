package fsnotify

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/abiosoft/colima/daemon/process"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
	"github.com/abiosoft/colima/environment/vm/lima/limautil"
	"github.com/abiosoft/colima/util"
	"github.com/sirupsen/logrus"
)

const Name = "fsnotify"

const watchInterval = time.Second * 2
const volumesInterval = time.Second * 5

// New returns fsnotify process.
func New() process.Process {
	return &fsnotifyProcess{
		interval: watchInterval,
		log:      *logrus.WithField("context", "notify"),
	}
}

var _ process.Process = (*fsnotifyProcess)(nil)

type fsnotifyProcess struct {
	alive         bool
	containerVols []string
	// will only be used for alive and containerVols
	sync.RWMutex

	interval time.Duration
	vmVols   []string
	guest    environment.GuestActions
	runtime  string

	log logrus.Entry
}

// Alive implements process.Process
func (f *fsnotifyProcess) Alive(ctx context.Context) error {
	f.Lock()
	defer f.RUnlock()

	if f.alive {
		return nil
	}
	return fmt.Errorf("fsnotify not running")
}

// Dependencies implements process.Process
func (*fsnotifyProcess) Dependencies() (deps []process.Dependency, root bool) {
	return nil, false
}

// Name implements process.Process
func (*fsnotifyProcess) Name() string {
	return Name
}

// Start implements process.Process
func (f *fsnotifyProcess) Start(ctx context.Context) error {
	log := f.log

	log.Trace("waiting for Lima to start")
	f.waitForLima(ctx)
	log.Trace("done waiting for Lima to start")

	c, err := limautil.InstanceConfig()
	if err != nil {
		return fmt.Errorf("error retrieving config")
	}
	f.runtime = c.Runtime
	f.guest = lima.New(host.New())

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
func (f *fsnotifyProcess) waitForLima(ctx context.Context) {
	log := f.log

	// wait for Lima to finish starting
	for {
		log.Trace("waiting for Lima...")

		// 5 second interval
		after := time.After(time.Second * 5)

		select {
		case <-ctx.Done():
			return
		case <-after:
			i, err := limautil.Instance()
			if err == nil && i.Running() {
				return
			}
		}
	}
}

func (f *fsnotifyProcess) watch(ctx context.Context) error {
	if err := f.fetchContainerVolumes(ctx); err != nil {
		return fmt.Errorf("error fetching container volumes")
	}

	return f.watchFiles(ctx)
}
