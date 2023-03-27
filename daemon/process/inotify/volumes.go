package inotify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/abiosoft/colima/environment/container/containerd"
	"github.com/abiosoft/colima/environment/container/docker"
)

func (f *inotifyProcess) monitorContainerVolumes(ctx context.Context, c chan<- []string) error {
	log := f.log

	if f.runtime == "" {
		return fmt.Errorf("empty runtime")
	}
	runtimeCmd := docker.Name
	if f.runtime == containerd.Name {
		runtimeCmd = "nerdctl"
	}

	fetch := func() ([]string, error) {
		// fetch all containers
		var containers []string
		{
			out, err := f.guest.RunOutput(runtimeCmd, "ps", "-q")
			if err != nil {
				return nil, fmt.Errorf("error listing containers: %w", err)
			}
			containers = strings.Fields(out)
			if len(containers) == 0 {
				return nil, nil
			}
		}

		log.Tracef("found containers %+v", containers)

		// fetch volumes
		var resp []struct {
			Mounts []struct {
				Source string `json:"Source"`
			} `json:"Mounts"`
		}
		{
			args := []string{runtimeCmd, "inspect"}
			args = append(args, containers...)

			var buf bytes.Buffer
			if err := f.guest.RunWith(nil, &buf, args...); err != nil {
				return nil, fmt.Errorf("error inspecting containers: %w", err)
			}
			if err := json.NewDecoder(&buf).Decode(&resp); err != nil {
				return nil, fmt.Errorf("error decoding docker response")
			}
		}

		// process and discard redundant volumes
		vols := []string{}
		{
			shouldMount := func(child string) bool {
				// ignore all invalid directories.
				// i.e. directories not within the mounted VM directories
				for _, parent := range f.vmVols {
					if strings.HasPrefix(child, parent) {
						return true
					}
				}
				return false
			}

			for _, r := range resp {
				for _, mount := range r.Mounts {
					if shouldMount(mount.Source) {
						vols = append(vols, mount.Source)
					}
				}
			}

			vols = omitChildrenDirectories(vols)
			log.Tracef("found volumes %+v", vols)
		}

		return vols, nil
	}

	go func() {
		for {
			select {
			case <-ctx.Done():
				log.Trace("stop signal recieved")
				err := ctx.Err()
				if err != nil {
					log.Trace(fmt.Errorf("error during stop: %w", err))
				}
			case <-time.After(volumesInterval):
				if vols, err := fetch(); err != nil {
					log.Error(err)
				} else {
					c <- vols
				}
			}
		}
	}()

	return nil
}

func omitChildrenDirectories(dirs []string) []string {
	sort.Strings(dirs) // sort to put the parent directories first

	// keep track for uniqueness
	set := map[string]struct{}{}

	var newVols []string

	omitted := map[int]struct{}{}
	for i := 0; i < len(dirs); i++ {
		// if the index is ommitted, skip
		if _, ok := omitted[i]; ok {
			continue
		}

		parent := dirs[i]
		if _, ok := set[parent]; !ok {
			newVols = append(newVols, parent)
			set[parent] = struct{}{}
		}

		for j := i + 1; j < len(dirs); j++ {
			child := dirs[j]
			if strings.HasPrefix(child, strings.TrimSuffix(parent, "/")+"/") {
				omitted[j] = struct{}{}
			}
		}
	}

	return newVols
}
