package main

import (
	"fmt"
	"os"
	"os/exec"
	"strings"
	"testing"
	"time"
)

// TestLoopbackPortForwarding tests that port forwarding works for different loopback addresses
func TestLoopbackPortForwarding(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// Test addresses that should work with our fix
	testAddresses := []string{
		"127.0.0.1",
		"127.0.0.5",
		"127.0.0.10",
	}

	for _, addr := range testAddresses {
		t.Run(fmt.Sprintf("address_%s", strings.ReplaceAll(addr, ".", "_")), func(t *testing.T) {
			// Start a simple HTTP server in a Docker container
			port := "8000"
			containerName := fmt.Sprintf("test-loopback-%s", strings.ReplaceAll(addr, ".", "-"))
			
			// Clean up any existing container
			exec.Command("docker", "rm", "-f", containerName).Run()
			
			// Start container with port binding to the specific address
			cmd := exec.Command("docker", "run", "-d", "--name", containerName,
				"-p", fmt.Sprintf("%s:%s:8000", addr, port),
				"python:3.8-slim", "python", "-m", "http.server", "8000")
			
			output, err := cmd.CombinedOutput()
			if err != nil {
				t.Fatalf("Failed to start container: %v\nOutput: %s", err, string(output))
			}
			
			// Give the container time to start
			time.Sleep(5 * time.Second)
			
			// Clean up the container when done
			defer exec.Command("docker", "rm", "-f", containerName).Run()
			
			// Try to connect to the server
			url := fmt.Sprintf("http://%s:%s", addr, port)
			curlCmd := exec.Command("curl", "-s", "--connect-timeout", "5", url)
			curlOutput, err := curlCmd.CombinedOutput()
			
			if err != nil {
				t.Errorf("Failed to connect to %s: %v\nOutput: %s", url, err, string(curlOutput))
				return
			}
			
			// Check if we got a valid HTTP response (should contain directory listing)
			if !strings.Contains(string(curlOutput), "Directory listing") && !strings.Contains(string(curlOutput), "<pre>") {
				t.Errorf("Unexpected response from %s: %s", url, string(curlOutput))
			}
			
			t.Logf("Successfully connected to %s", url)
		})
	}
}

// This can be run manually to test the fix
func main() {
	if len(os.Args) > 1 && os.Args[1] == "test" {
		testing.Main(func(pat, str string) (bool, error) { return true, nil },
			[]testing.InternalTest{
				{
					Name: "TestLoopbackPortForwarding",
					F:    TestLoopbackPortForwarding,
				},
			},
			nil, nil)
	}
}
