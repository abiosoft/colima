package vm

import (
	"fmt"
	"log"
	"sync"

	"github.com/abiosoft/colima/environment"
)

// Backend is the type of VM backend.
type Backend string

// VM backend constants.
const (
	BackendLima  Backend = "lima"
	BackendApple Backend = "apple"
)

// NewVMFunc is implemented by VM backend implementations to create a new instance.
type NewVMFunc func(host environment.HostActions) environment.VM

// InstanceInfo contains information about a VM instance.
type InstanceInfo struct {
	Name      string `json:"name,omitempty"`
	Status    string `json:"status,omitempty"`
	Arch      string `json:"arch,omitempty"`
	CPU       int    `json:"cpus,omitempty"`
	Memory    int64  `json:"memory,omitempty"`
	Disk      int64  `json:"disk,omitempty"`
	Dir       string `json:"dir,omitempty"`
	IPAddress string `json:"address,omitempty"`
	Runtime   string `json:"runtime,omitempty"`
	Backend   string `json:"backend,omitempty"`
}

// Running checks if the instance is running.
func (i InstanceInfo) Running() bool { return i.Status == "Running" }

// InstanceLister is implemented by VM backends to list their instances.
type InstanceLister interface {
	// Instances returns all instances managed by this backend.
	Instances(ids ...string) ([]InstanceInfo, error)
}

var (
	vmBackends      = map[Backend]NewVMFunc{}
	instanceListers = map[Backend]InstanceLister{}
	mu              sync.RWMutex
)

// RegisterVM registers a new VM backend.
func RegisterVM(backend Backend, f NewVMFunc) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := vmBackends[backend]; ok {
		log.Fatalf("vm backend '%s' already registered", backend)
	}
	vmBackends[backend] = f
}

// RegisterInstanceLister registers an instance lister for a VM backend.
func RegisterInstanceLister(backend Backend, lister InstanceLister) {
	mu.Lock()
	defer mu.Unlock()

	if _, ok := instanceListers[backend]; ok {
		log.Fatalf("instance lister for backend '%s' already registered", backend)
	}
	instanceListers[backend] = lister
}

// NewVM creates a new VM for the specified backend.
func NewVM(backend Backend, host environment.HostActions) (environment.VM, error) {
	mu.RLock()
	defer mu.RUnlock()

	f, ok := vmBackends[backend]
	if !ok {
		return nil, fmt.Errorf("unsupported VM backend '%s'", backend)
	}
	return f(host), nil
}

// AllInstances returns all instances from all registered backends.
func AllInstances(ids ...string) ([]InstanceInfo, error) {
	mu.RLock()
	defer mu.RUnlock()

	var all []InstanceInfo
	for backend, lister := range instanceListers {
		instances, err := lister.Instances(ids...)
		if err != nil {
			// Log the error but continue with other backends
			log.Printf("error retrieving instances from backend '%s': %v", backend, err)
			continue
		}
		// Ensure backend is set on each instance
		for i := range instances {
			if instances[i].Backend == "" {
				instances[i].Backend = string(backend)
			}
		}
		all = append(all, instances...)
	}
	return all, nil
}

// Backends returns all registered backend names.
func Backends() []Backend {
	mu.RLock()
	defer mu.RUnlock()

	backends := make([]Backend, 0, len(vmBackends))
	for b := range vmBackends {
		backends = append(backends, b)
	}
	return backends
}
