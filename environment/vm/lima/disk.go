package lima

import (
	"encoding/json"
	"fmt"
	"github.com/abiosoft/colima/config"
	"github.com/docker/go-units"
	"github.com/sirupsen/logrus"
	"strconv"
	"strings"
)

func (l *limaVM) updateLimaDisks(conf config.Config) ([]DiskListOutput, error) {
	logrus.Trace(fmt.Errorf("updating lima disks"))
	out, err := l.host.RunOutput(limactl, "disk", "list", "--json")
	if err != nil {
		logrus.Trace(fmt.Errorf("error listing disks: %s, %s", out, err))
		return []DiskListOutput{}, err
	}
	logrus.Trace(fmt.Errorf("listing disks: %s", out))

	var listedDisks []DiskListOutput
	if out != "" {
		for _, line := range strings.Split(strings.TrimSuffix(out, "\n"), "\n") {
			var listedDisk DiskListOutput
			err = json.Unmarshal([]byte(line), &listedDisk)
			if err != nil {
				logrus.Trace(fmt.Errorf("error unmarshaling listed disks: %s", err))
				return []DiskListOutput{}, err
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
		diskFormat := "--format=raw"
		if disk.FSType != "" {
			diskFormat = "--format=" + disk.FSType
		}
		out, err = l.host.RunOutput(limactl, "disk", "create", diskName, "--size", disk.Size, diskFormat)
		if err != nil {
			logrus.Trace(fmt.Errorf("error creating disk: %s, %s", out, err))
			return []DiskListOutput{}, err
		}
		logrus.Trace(fmt.Errorf("disk create output: %s", out))
	}

	var disksToDelete []DiskListOutput
	for _, listedDisk := range listedDisks {
		var found = false
		for _, disk := range conf.LimaDisks {
			diskName := config.CurrentProfile().ID + "-" + disk.Name
			if listedDisk.Name == diskName {
				found = true
				diskSize, err := units.RAMInBytes(disk.Size)
				if err != nil {
					logrus.Trace(fmt.Errorf("error parsing disk size: %s", err))
					return []DiskListOutput{}, err
				}
				logrus.Trace(fmt.Errorf("disk size: %d", diskSize))
				switch disk.FSType {
				case "raw":
					if diskSize != listedDisk.Size {
						l.resizeLimaDisk(diskName, diskSize)
					}
				case "qcow2":
					if diskSize != 0 {
						return []DiskListOutput{}, fmt.Errorf("%s cannot be updated: limactl does not support updating disks", diskName)
					}
				}
				logrus.Trace(fmt.Errorf("disk %s is up to date", diskName))
				continue

			}
		}
		if !found {
			disksToDelete = append(disksToDelete, listedDisk)
		}
	}

	for _, disk := range disksToDelete {
		l.deleteLimaDisk(disk.Name)
	}

	return disksToDelete, nil
}

func (l *limaVM) resizeLimaDisk(diskName string, diskSize int64) {
	out, err := l.host.RunOutput(limactl, "disk", "resize", diskName, "--size", strconv.FormatInt(diskSize, 10))
	if err != nil {
		logrus.Trace(fmt.Errorf("error resizing disk: %s, %s", out, err))
		return
	}
	logrus.Trace(fmt.Errorf("disk resize output: %s", out))
}

func (l *limaVM) deleteLimaDisk(diskName string) {
	logrus.Trace(fmt.Errorf("deleting disk %s", diskName))
	out, err := l.host.RunOutput(limactl, "disk", "delete", "--force", diskName)
	if err != nil {
		logrus.Trace(fmt.Errorf("error deleting disk: %s, %s", out, err))
		return
	}
	logrus.Trace(fmt.Errorf("disk delete output: %s", out))
}
