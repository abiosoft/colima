package apple

import (
	"testing"

	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/host"
	"github.com/abiosoft/colima/environment/vm/lima"
)

func TestAppleRuntime_Interface(t *testing.T) {
	// Test that appleRuntime implements the Container interface
	var _ environment.Container = (*appleRuntime)(nil)
}

func TestAppleRuntime_Name(t *testing.T) {
	host := host.New()
	guest := lima.New(host)
	runtime := newRuntime(host, guest)
	
	if runtime.Name() != Name {
		t.Errorf("Expected name to be %s, got %s", Name, runtime.Name())
	}
}

func TestAppleRuntime_Dependencies(t *testing.T) {
	host := host.New()
	guest := lima.New(host)
	runtime := newRuntime(host, guest)
	
	deps := runtime.Dependencies()
	if len(deps) != 0 {
		t.Errorf("Expected no dependencies for Apple Container, got %v", deps)
	}
}

func TestAppleRuntime_Registration(t *testing.T) {
	// Test that Apple Container is registered
	runtimes := environment.ContainerRuntimes()
	found := false
	for _, runtime := range runtimes {
		if runtime == Name {
			found = true
			break
		}
	}
	
	if !found {
		t.Errorf("Apple Container runtime not found in registered runtimes: %v", runtimes)
	}
} 