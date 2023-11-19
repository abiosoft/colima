package lima

import (
	"context"
	"fmt"
	"time"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/daemon"
	"github.com/abiosoft/colima/daemon/process/inotify"
	"github.com/abiosoft/colima/daemon/process/vmnet"
	"github.com/abiosoft/colima/util"
)

func (l *limaVM) startDaemon(ctx context.Context, conf config.Config) (context.Context, error) {
	isQEMU := conf.VMType == QEMU
	isVZ := conf.VMType == VZ

	// limited to macOS (with Qemu driver)
	// or vz with inotify enabled
	if !util.MacOS() || (isVZ && !conf.MountINotify) {
		return ctx, nil
	}

	ctxKeyVmnet := daemon.CtxKey(vmnet.Name)
	ctxKeyInotify := daemon.CtxKey(inotify.Name)

	// use a nested chain for convenience
	a := l.Init(ctx)
	log := l.Logger(ctx)

	networkInstalledKey := struct{ key string }{key: "network_installed"}

	// add inotify to daemon
	if conf.MountINotify {
		a.Add(func() error {
			ctx = context.WithValue(ctx, ctxKeyInotify, true)
			deps, _ := l.daemon.Dependencies(ctx, conf)
			if err := deps.Install(l.host); err != nil {
				return fmt.Errorf("error setting up inotify dependencies: %w", err)
			}
			return nil
		})
	}

	// add network processes to daemon
	if isQEMU {
		a.Add(func() error {
			if conf.Network.Address {
				a.Stage("preparing network")
				ctx = context.WithValue(ctx, ctxKeyVmnet, true)
			}
			deps, root := l.daemon.Dependencies(ctx, conf)
			if deps.Installed() {
				ctx = context.WithValue(ctx, networkInstalledKey, true)
				return nil
			}

			// if user interaction is not required (i.e. root),
			// no need for another verbose info.
			if root {
				log.Println("dependencies missing for setting up reachable IP address")
				log.Println("sudo password may be required")
			}

			// install deps
			err := deps.Install(l.host)
			if err != nil {
				ctx = context.WithValue(ctx, networkInstalledKey, false)
			}
			return err
		})
	}

	// start daemon
	a.Add(func() error {
		return l.daemon.Start(ctx, conf)
	})

	statusKey := struct{ key string }{key: "daemonStatus"}
	// delay to ensure that the processes have started
	if conf.Network.Address || conf.MountINotify {
		a.Retry("", time.Second*1, 15, func(i int) error {
			s, err := l.daemon.Running(ctx, conf)
			ctx = context.WithValue(ctx, statusKey, s)
			if err != nil {
				return err
			}
			if !s.Running {
				return fmt.Errorf("daemon is not running")
			}
			for _, p := range s.Processes {
				if !p.Running {
					return p.Error
				}
			}
			return nil
		})
	}

	// network failure is not fatal
	if err := a.Exec(); err != nil {
		if isQEMU {
			func() {
				installed, _ := ctx.Value(networkInstalledKey).(bool)
				if !installed {
					log.Warnln(fmt.Errorf("error setting up network dependencies: %w", err))
					return
				}

				status, ok := ctx.Value(statusKey).(daemon.Status)
				if !ok {
					return
				}
				if !status.Running {
					log.Warnln(fmt.Errorf("error starting network: %w", err))
					return
				}

				for _, p := range status.Processes {
					// TODO: handle inotify separate from network
					if p.Name == inotify.Name {
						continue
					}
					if !p.Running {
						ctx = context.WithValue(ctx, daemon.CtxKey(p.Name), false)
						log.Warnln(fmt.Errorf("error starting %s: %w", p.Name, err))
					}
				}
			}()
		}
	}

	// check if inotify is running
	if conf.MountINotify {
		if inotifyEnabled, _ := ctx.Value(ctxKeyInotify).(bool); !inotifyEnabled {
			log.Warnln("error occurred enabling inotify daemon")
		}
	}

	// preserve vmnet context
	if vmnetEnabled, _ := ctx.Value(ctxKeyVmnet).(bool); vmnetEnabled {
		// env var for subprocess to detect vmnet
		l.host = l.host.WithEnv(vmnet.SubProcessEnvVar + "=1")
	}

	return ctx, nil
}
