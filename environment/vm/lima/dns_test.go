package lima

import (
	"context"
	"errors"
	"io"
	"net"
	"os"
	"strings"
	"testing"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment"
)

// fakeDnsmasqRunner is the minimal package-private seam for the dnsmasq
// capability probe: it makes command result, stdout, and error observable.
type fakeDnsmasqRunner struct {
	out  string
	err  error
	args []string
}

func (f *fakeDnsmasqRunner) RunOutput(args ...string) (string, error) {
	f.args = append(f.args, args...)
	return f.out, f.err
}

func TestDNSMasqCapabilityRejectsUnknownOrMalformedState(t *testing.T) {
	tests := []struct {
		name string
		out  string
	}{
		{name: "empty", out: ""},
		{name: "whitespace only", out: "   \n\t "},
		{name: "multiline", out: "loaded\nnot-found"},
		{name: "unknown state", out: "activating"},
		{name: "transient state", out: "reloading"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeDnsmasqRunner{out: tt.out}
			present, err := hasDnsmasq(r)
			if err == nil {
				t.Fatalf("hasDnsmasq() = present=%v, err=nil; want error for load state %q", present, tt.out)
			}
			if present {
				t.Fatalf("hasDnsmasq() present=%v; want false for load state %q", present, tt.out)
			}
		})
	}
}

func TestDNSMasqCapabilityPropagatesCommandFailure(t *testing.T) {
	tests := []struct {
		name string
		err  error
	}{
		{name: "transport failure", err: errors.New("connection refused")},
		{name: "permission failure", err: errors.New("permission denied")},
		{name: "nonzero exit", err: errors.New("exit status 1")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeDnsmasqRunner{err: tt.err}
			present, err := hasDnsmasq(r)
			if !errors.Is(err, tt.err) {
				t.Fatalf("hasDnsmasq() err = %v; want %v", err, tt.err)
			}
			if present {
				t.Fatalf("hasDnsmasq() present = %v; want false", present)
			}
		})
	}
}

func TestDNSMasqCapabilityIgnoresPackageUpgradePresentation(t *testing.T) {
	r := &fakeDnsmasqRunner{out: "loaded"}
	present, err := hasDnsmasq(r)
	if err != nil {
		t.Fatalf("hasDnsmasq() err = %v; want nil", err)
	}
	if !present {
		t.Fatal("hasDnsmasq() present = false; want true for loaded service")
	}
	want := "systemctl show dnsmasq.service --property=LoadState --value"
	if got := strings.Join(r.args, " "); got != want {
		t.Fatalf("probe command = %q; want %q", got, want)
	}
}

// TestDNSMasqCapabilityExactStates characterizes the exact tri-state contract:
// an exact "loaded" state is present, an exact "not-found" state is absent.
// A guest image with the dnsmasq executable but no loadable service unit
// (base-only, e.g. dnsmasq-base without the service) still reports not-found,
// so the probe never treats the binary as capability.
func TestDNSMasqCapabilityExactStates(t *testing.T) {
	tests := []struct {
		name    string
		out     string
		present bool
	}{
		{name: "loaded service is present", out: "loaded", present: true},
		{name: "not-found service is absent", out: "not-found", present: false},
		{name: "loaded with trailing newline", out: "loaded\n", present: true},
		{name: "not-found with trailing whitespace", out: "not-found \n", present: false},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &fakeDnsmasqRunner{out: tt.out}
			present, err := hasDnsmasq(r)
			if err != nil {
				t.Fatalf("hasDnsmasq() err = %v; want nil", err)
			}
			if present != tt.present {
				t.Fatalf("hasDnsmasq() present = %v; want %v", present, tt.present)
			}
		})
	}
}

// TestSetupDNSPresentServiceContinuation characterizes the present-service
// path: a loaded service enters the existing DNS setup sequence, config and
// resolver writes precede the service restart, and setupDNS returns nil.
//
// PATH is emptied so the guest IP acquisition helper fails instantly instead
// of invoking a real limactl; the helper swallows that error and returns "".
func TestSetupDNSPresentServiceContinuation(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	h := &fakeHost{runOutput: func(args ...string) (string, error) {
		return "loaded", nil
	}}
	l := &limaVM{host: h, CommandChain: cli.New("vm")}

	if err := l.setupDNS(config.Config{}); err != nil {
		t.Fatalf("setupDNS() err = %v; want nil", err)
	}

	joined := func(call []string) string { return strings.Join(call, " ") }
	writeIdx, restartIdx := -1, -1
	for i, call := range h.calls {
		s := joined(call)
		if strings.HasSuffix(s, "sh -c cat > /etc/dnsmasq.d/01-colima.conf") {
			writeIdx = i
		}
		if strings.HasSuffix(s, "systemctl restart dnsmasq") {
			restartIdx = i
		}
	}
	if writeIdx < 0 {
		t.Fatalf("setupDNS() did not write dnsmasq config; calls: %v", h.calls)
	}
	if restartIdx < 0 {
		t.Fatalf("setupDNS() did not restart dnsmasq; calls: %v", h.calls)
	}
	if writeIdx > restartIdx {
		t.Fatalf("setupDNS() wrote dnsmasq config after restart (write idx %d, restart idx %d); want config write before restart", writeIdx, restartIdx)
	}

	wantProbe := "lima systemctl show dnsmasq.service --property=LoadState --value"
	if got := joined(h.calls[0]); got != wantProbe {
		t.Fatalf("first command = %q; want probe %q", got, wantProbe)
	}
}

// TestSetupDNSPresentPathPropagatesSequenceFailure characterizes the
// present-service sequence error contract: once the probe reports the service
// present, a failure in the DNS setup sequence (here the dnsmasq restart)
// propagates out of setupDNS instead of being silently swallowed.
func TestSetupDNSPresentPathPropagatesSequenceFailure(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	restartErr := errors.New("systemctl restart failed")
	h := &fakeHost{
		runOutput: func(args ...string) (string, error) {
			return "loaded", nil
		},
		runQuiet: func(args ...string) error {
			if len(args) >= 2 && args[len(args)-2] == "restart" && args[len(args)-1] == "dnsmasq" {
				return restartErr
			}
			return nil
		},
	}
	l := &limaVM{host: h, CommandChain: cli.New("vm")}

	err := l.setupDNS(config.Config{})
	if err == nil {
		t.Fatal("setupDNS() err = nil; want restart failure")
	}
	if !strings.Contains(err.Error(), restartErr.Error()) {
		t.Fatalf("setupDNS() err = %v; want it to contain %v", err, restartErr)
	}
	if !strings.Contains(err.Error(), "failed to restart dnsmasq service") {
		t.Fatalf("setupDNS() err = %v; want it to contain %q", err, "failed to restart dnsmasq service")
	}
}

// TestAddPostStartActionsSetupDNSFirst characterizes post-start ordering
// (lima.go addPostStartActions): setupDNS is the first post-start action, and
// its error prevents later post-start actions from running.
func TestAddPostStartActionsSetupDNSFirst(t *testing.T) {
	probeErr := errors.New("exit status 1")
	h := &fakeHost{runOutput: func(args ...string) (string, error) {
		return "", probeErr
	}}
	l := &limaVM{host: h, CommandChain: cli.New("vm")}

	a := l.Init(context.Background())
	l.addPostStartActions(a, config.Config{})
	err := a.Exec()
	if err == nil {
		t.Fatal("Exec() err = nil; want setupDNS error")
	}
	if !strings.Contains(err.Error(), "error setting up DNS") {
		t.Fatalf("Exec() err = %v; want it to contain %q", err, "error setting up DNS")
	}
	if len(h.calls) != 1 {
		t.Fatalf("Exec() ran %d guest commands; want only the capability probe (later post-start actions blocked)", len(h.calls))
	}
}

// fakeHost is a package-private fake of environment.HostActions that records
// every command invocation and lets tests drive the probe result.
type fakeHost struct {
	runOutput func(args ...string) (string, error)
	runQuiet  func(args ...string) error
	runWith   func(stdin io.Reader, args ...string) error
	calls     [][]string
	written   map[string][]byte
}

func (f *fakeHost) record(args []string) { f.calls = append(f.calls, args) }

func (f *fakeHost) Run(args ...string) error { f.record(args); return nil }
func (f *fakeHost) RunQuiet(args ...string) error {
	f.record(args)
	if f.runQuiet != nil {
		return f.runQuiet(args...)
	}
	return nil
}
func (f *fakeHost) RunOutput(args ...string) (string, error) {
	f.record(args)
	if f.runOutput != nil {
		return f.runOutput(args...)
	}
	return "", nil
}
func (f *fakeHost) RunInteractive(args ...string) error { f.record(args); return nil }
func (f *fakeHost) RunWith(stdin io.Reader, stdout io.Writer, args ...string) error {
	f.record(args)
	if f.runWith != nil {
		return f.runWith(stdin, args...)
	}
	if stdin != nil {
		body, err := io.ReadAll(stdin)
		if err != nil {
			return err
		}
		if f.written == nil {
			f.written = map[string][]byte{}
		}
		for _, a := range args {
			if target, ok := strings.CutPrefix(a, "cat > "); ok {
				f.written[target] = body
				break
			}
		}
	}
	return nil
}
func (f *fakeHost) Read(fileName string) (string, error) { return "", nil }
func (f *fakeHost) Write(fileName string, body []byte) error {
	return nil
}
func (f *fakeHost) Stat(fileName string) (os.FileInfo, error) { return nil, nil }
func (f *fakeHost) WithEnv(env ...string) environment.HostActions {
	return f
}
func (f *fakeHost) WithDir(dir string) environment.HostActions { return f }
func (f *fakeHost) Env(string) string                          { return "" }

func TestSetupDNSNoopsOnlyWhenServiceIsConfirmedAbsent(t *testing.T) {
	h := &fakeHost{runOutput: func(args ...string) (string, error) {
		return "not-found", nil
	}}
	l := &limaVM{host: h, CommandChain: cli.New("vm")}

	if err := l.setupDNS(config.Config{}); err != nil {
		t.Fatalf("setupDNS() err = %v; want nil", err)
	}
	if len(h.calls) != 1 {
		t.Fatalf("setupDNS() executed %d guest commands; want only the capability probe", len(h.calls))
	}
	want := "lima systemctl show dnsmasq.service --property=LoadState --value"
	if got := strings.Join(h.calls[0], " "); got != want {
		t.Fatalf("probe command = %q; want %q", got, want)
	}
}

func TestSetupDNSPropagatesCapabilityProbeError(t *testing.T) {
	probeErr := errors.New("exit status 1")
	h := &fakeHost{runOutput: func(args ...string) (string, error) {
		return "", probeErr
	}}
	l := &limaVM{host: h, CommandChain: cli.New("vm")}

	err := l.setupDNS(config.Config{})
	if err == nil {
		t.Fatal("setupDNS() err = nil; want capability probe error")
	}
	if !strings.Contains(err.Error(), probeErr.Error()) {
		t.Fatalf("setupDNS() err = %v; want it to contain %v", err, probeErr)
	}
	if len(h.calls) != 1 {
		t.Fatalf("setupDNS() executed %d guest commands; want only the capability probe", len(h.calls))
	}
}

// TestSetupDNSRendersCustomGatewayHostnameAndResolvers covers the remaining
// setupDNS rendering branches: custom gateway address, hostname entry, and
// explicit DNSResolvers taking precedence over the default gateway server.
func TestSetupDNSRendersCustomGatewayHostnameAndResolvers(t *testing.T) {
	t.Setenv("PATH", t.TempDir())

	gateway := net.ParseIP("10.0.0.2")
	hostname := "myhost"
	resolvers := []net.IP{net.ParseIP("1.1.1.1"), net.ParseIP("9.9.9.9")}
	h := &fakeHost{runOutput: func(args ...string) (string, error) {
		return "loaded", nil
	}}
	l := &limaVM{host: h, CommandChain: cli.New("vm")}

	conf := config.Config{
		Hostname: hostname,
		Network: config.Network{
			GatewayAddress: gateway,
			DNSResolvers:   resolvers,
		},
	}
	if err := l.setupDNS(conf); err != nil {
		t.Fatalf("setupDNS() err = %v; want nil", err)
	}
	confOut := string(h.written["/etc/dnsmasq.d/01-colima.conf"])
	for _, want := range []string{
		"address=/host.docker.internal/10.0.0.2",
		"address=/host.lima.internal/10.0.0.2",
		"address=/myhost/127.0.0.1",
		"server=1.1.1.1",
		"server=9.9.9.9",
	} {
		if !strings.Contains(confOut, want) {
			t.Fatalf("dnsmasq config missing %q:\n%s", want, confOut)
		}
	}
	if strings.Contains(confOut, "server=10.0.0.2") {
		t.Fatalf("dnsmasq config has default gateway server despite explicit resolvers:\n%s", confOut)
	}
}

// TestSetupDNSPresentPathPropagatesWriteFailure covers the sequence failure
// branches: mkdir, dnsmasq config write, resolv.conf removal, and resolv.conf
// write failures all propagate out of setupDNS.
func TestSetupDNSPresentPathPropagatesWriteFailure(t *testing.T) {
	tests := []struct {
		name     string
		runQuiet func(args ...string) error
		runWith  func(stdin io.Reader, args ...string) error
		wantErr  string
	}{
		{
			name: "mkdir fails",
			runQuiet: func(args ...string) error {
				for _, a := range args {
					if a == "mkdir" {
						return errors.New("mkdir failed")
					}
				}
				return nil
			},
			wantErr: "failed to create dnsmasq config directory",
		},
		{
			name: "dnsmasq config write fails",
			runWith: func(stdin io.Reader, args ...string) error {
				for _, a := range args {
					if strings.Contains(a, "01-colima.conf") {
						return errors.New("write failed")
					}
				}
				return nil
			},
			wantErr: "failed to write dnsmasq config",
		},
		{
			name: "resolv.conf removal fails",
			runQuiet: func(args ...string) error {
				for _, a := range args {
					if a == "rm" {
						return errors.New("rm failed")
					}
				}
				return nil
			},
			wantErr: "failed to remove existing resolv.conf",
		},
		{
			name: "resolv.conf write fails",
			runWith: func(stdin io.Reader, args ...string) error {
				for _, a := range args {
					if strings.Contains(a, "resolv.conf") && strings.Contains(a, "cat >") {
						return errors.New("write failed")
					}
				}
				return nil
			},
			wantErr: "failed to write resolv.conf",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("PATH", t.TempDir())
			h := &fakeHost{runOutput: func(args ...string) (string, error) {
				return "loaded", nil
			}}
			h.runQuiet = tt.runQuiet
			h.runWith = tt.runWith
			l := &limaVM{host: h, CommandChain: cli.New("vm")}

			err := l.setupDNS(config.Config{})
			if err == nil {
				t.Fatal("setupDNS() err = nil; want sequence failure")
			}
			if !strings.Contains(err.Error(), tt.wantErr) {
				t.Fatalf("setupDNS() err = %v; want it to contain %q", err, tt.wantErr)
			}
		})
	}
}
