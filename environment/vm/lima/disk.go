package lima

import (
	"encoding/json"
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"strings"
)

func (l *limaVM) updateLimaDisks(conf config.Config) ([]Disk, error) {
	logrus.Trace(fmt.Errorf("updating lima disks: %s", conf.LimaDisks))
	out, err := l.host.RunOutput(limactl, "disk", "list", "--json")
	if err != nil {
		logrus.Trace(fmt.Errorf("error listing disks: %s, %s", out, err))
		return []Disk{}, err
	}
	logrus.Trace(fmt.Errorf("listing disks: %s", out))

	var listedDisks []Disk
	if out != "" {
		for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
			var listedDisk Disk
			err = json.Unmarshal([]byte(line), &listedDisk)
			if err != nil {
				logrus.Trace(fmt.Errorf("error unmarshaling listed disks: %s", err))
				return []Disk{}, err
			}
			listedDisks = append(listedDisks, listedDisk)
		}
	}

	var disksToCreate []config.Disk
	for _, disk := range conf.LimaDisks {
		diskName := config.CurrentProfile().ID + "-" + disk.Name
		var found = false
		for _, listedDisk := range listedDisks {
			if listedDisk.Name == diskName {
				found = true
				break
			}
		}
		if !found {
			disksToCreate = append(disksToCreate, disk)
		}
	}

	for _, disk := range disksToCreate {
		diskName := config.CurrentProfile().ID + "-" + disk.Name
		logrus.Trace(fmt.Errorf("creating disk %s", diskName))
		out, err = l.host.RunOutput(limactl, "disk", "create", diskName, "--size", disk.Size, "--format", "raw")
		if err != nil {
			logrus.Trace(fmt.Errorf("error creating disk: %s, %s", out, err))
			return []Disk{}, err
		}
		logrus.Trace(fmt.Errorf("disk create output: %s", out))
	}

	var disksToDelete []Disk
	for _, listedDisk := range listedDisks {
		var found = false
		for _, disk := range conf.LimaDisks {
			diskName := config.CurrentProfile().ID + "-" + disk.Name
			if listedDisk.Name == diskName {
				found = true
				diskSize, err := units.RAMInBytes(disk.Size)
				if err != nil {
					logrus.Trace(fmt.Errorf("error parsing disk size: %s", err))
					return []Disk{}, err
				}
				logrus.Trace(fmt.Errorf("disk size: %d", diskSize))

				if diskSize == listedDisk.Size {
					logrus.Trace(fmt.Errorf("disk %s is up to date", diskName))
					continue
				}
				return []Disk{}, fmt.Errorf("%s cannot be updated: limactl does not support updating disks", diskName)
			}
		}
		if !found {
			disksToDelete = append(disksToDelete, listedDisk)
		}
	}

	for _, disk := range disksToDelete {
		logrus.Trace(fmt.Errorf("deleting disk %s", disk.Name))
		out, err := l.host.RunOutput(limactl, "disk", "delete", disk.Name)
		if err != nil {
			logrus.Trace(fmt.Errorf("error deleting disk: %s, %s", out, err))
			return []Disk{}, err
		}
		logrus.Trace(fmt.Errorf("disk delete output: %s", out))
	}

	return disksToDelete, nil
}
