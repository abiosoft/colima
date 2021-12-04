package podman

import (
	"encoding/json"
	"fmt"
	"strings"
)

type limaVM struct {
	Name         string
	Status       string
	Dir          string
	Arch         string
	SSHLocalPort int
	HostAgentPID int
	QemuPID      int
}

func (p podmanRuntime) getSSHPortFromLimactl() (int, error) {
	// get ssh port from limactl since sshport environment seems to be always different
	limaVMJSON, err := p.host.RunOutput("limactl", "list", "--json")
	if err != nil {
		return 0, fmt.Errorf("Can't get lima VMs on host: %v", err)
	}
	for _, vmJSON := range strings.Split(limaVMJSON, "\n") {
		var vm limaVM
		err = json.Unmarshal([]byte(vmJSON), &vm)
		if err != nil {
			return 0, fmt.Errorf("Can't unmarshal lima VMs json %v", err)
		}
		if vm.Name == "colima" {
			return vm.SSHLocalPort, nil
		}
	}
	return 0, fmt.Errorf("Colima VM wasn't found in lima vms")
}

type podmanConnections struct {
	Name     string
	Identity string
	URI      string
}
