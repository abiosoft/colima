package lima

import (
	"context"
	_ "embed"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/environment/vm/lima/limaconfig"
	"github.com/abiosoft/colima/util"
)

//go:embed physical_disk.sh
var physicalDiskScript string

const (
	physicalDiskGuestNBDPortBase = 10809
	physicalDiskNFSPort          = 2049
	physicalDiskNFSRoot          = "/mnt/colima/physical"
)

type physicalDiskRuntime struct {
	config.PhysicalDisk

	index        int
	nbdDevice    string
	guestDevice  string
	nbdGuestPort int
	nbdHostPort  int
	nfsHostPort  int
	stateDir     string
}

func (l *limaVM) setupPhysicalDisks(ctx context.Context, conf config.Config) error {
	if len(conf.PhysicalDisks) == 0 {
		return nil
	}
	if !util.MacOS() {
		return fmt.Errorf("physical disks are only supported on macOS")
	}

	var runtimes []physicalDiskRuntime
	requiresNBD := false
	for i, disk := range conf.PhysicalDisks {
		runtime, err := newPhysicalDiskRuntime(i, disk)
		if err != nil {
			return err
		}
		runtimes = append(runtimes, runtime)
		if runtime.Backend == "nbd" {
			requiresNBD = true
		}
	}

	if requiresNBD {
		if err := util.AssertQemuNBD(); err != nil {
			return err
		}
	}

	for _, runtime := range runtimes {
		if err := l.setupPhysicalDisk(ctx, runtime); err != nil {
			return err
		}
	}
	return nil
}

func (l *limaVM) assertPhysicalDiskBackends(conf config.Config) error {
	for _, disk := range conf.PhysicalDisks {
		if physicalDiskBackend(disk) != "vz" {
			continue
		}
		if l.limaConf.VMType != limaconfig.VZ {
			return fmt.Errorf("physicalDisks.%s backend vz requires vmType: vz", disk.Name)
		}
		if !util.MacOS14OrNewer() {
			return fmt.Errorf("physicalDisks.%s backend vz requires macOS 14 or newer", disk.Name)
		}
		if !l.limaSupportsSafeVZBlockDevices() {
			return fmt.Errorf("physicalDisks.%s backend vz requires a Lima build with secure macOS VZ block-device support; expected limactl create --help and limactl sudoers --help to expose --block-device", disk.Name)
		}
	}
	return nil
}

func (l *limaVM) limaSupportsSafeVZBlockDevices() bool {
	createHelp, err := l.host.RunOutput(limactl, "create", "--help")
	if err != nil || !strings.Contains(createHelp, "--block-device") {
		return false
	}
	sudoersHelp, err := l.host.RunOutput(limactl, "sudoers", "--help")
	return err == nil && strings.Contains(sudoersHelp, "--block-device")
}

func (l *limaVM) setupPhysicalDisk(ctx context.Context, disk physicalDiskRuntime) error {
	// Clear stale state from a previous interrupted startup.
	if err := l.stopPhysicalDisk(ctx, disk); err != nil {
		l.Logger(ctx).Warnln(fmt.Errorf("error cleaning up physical disk %s: %w", disk.Name, err))
	}

	cleanup := true
	defer func() {
		if cleanup {
			_ = l.stopPhysicalDisk(ctx, disk)
		}
	}()

	if err := l.assertPhysicalDiskReady(disk); err != nil {
		return fmt.Errorf("physical disk %s is not ready: %w", disk.Name, err)
	}
	switch disk.Backend {
	case "nbd":
		if err := l.startPhysicalDiskNBD(disk); err != nil {
			return fmt.Errorf("error starting physical disk %s NBD backend: %w", disk.Name, err)
		}
		if err := l.startPhysicalDiskNBDTunnel(disk); err != nil {
			return fmt.Errorf("error starting physical disk %s NBD tunnel: %w", disk.Name, err)
		}
		if err := l.attachPhysicalDiskInGuest(disk, disk.nbdDevice); err != nil {
			return fmt.Errorf("error attaching physical disk %s in VM: %w", disk.Name, err)
		}
	case "vz":
		if err := l.attachPhysicalDiskInGuest(disk, disk.guestDevice); err != nil {
			return fmt.Errorf("error mounting physical disk %s in VM: %w", disk.Name, err)
		}
	default:
		return fmt.Errorf("unsupported physical disk backend %q", disk.Backend)
	}
	if err := l.assertPhysicalDiskBackendLive(disk); err != nil {
		return fmt.Errorf("error checking physical disk %s backend health: %w", disk.Name, err)
	}
	if disk.HostAccess.Enabled {
		if err := l.startPhysicalDiskNFSTunnel(disk); err != nil {
			return fmt.Errorf("error starting physical disk %s NFS tunnel: %w", disk.Name, err)
		}
		if err := l.mountPhysicalDiskOnHost(disk); err != nil {
			return fmt.Errorf("error mounting physical disk %s on host: %w", disk.Name, err)
		}
		if err := l.assertPhysicalDiskHostAccessLive(disk); err != nil {
			return fmt.Errorf("error checking physical disk %s host access health: %w", disk.Name, err)
		}
	}

	cleanup = false
	return nil
}

func (l *limaVM) stopPhysicalDisks(ctx context.Context, conf config.Config) error {
	var errs []string
	for i, disk := range conf.PhysicalDisks {
		runtime, err := newPhysicalDiskRuntime(i, disk)
		if err != nil {
			errs = append(errs, err.Error())
			continue
		}
		if err := l.stopPhysicalDisk(ctx, runtime); err != nil {
			errs = append(errs, fmt.Sprintf("%s: %v", runtime.Name, err))
		}
	}
	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func newPhysicalDiskRuntime(index int, disk config.PhysicalDisk) (physicalDiskRuntime, error) {
	if disk.RawDevice == "" {
		disk.RawDevice = strings.Replace(disk.Device, "/dev/disk", "/dev/rdisk", 1)
	}
	if disk.FSType == "" {
		disk.FSType = "auto"
	}
	disk.Backend = physicalDiskBackend(disk)
	if disk.MountPoint == "" {
		disk.MountPoint = filepath.Join("/mnt/colima/physical", disk.Name)
	}
	if disk.HostAccess.Enabled {
		if disk.HostAccess.Driver == "" {
			disk.HostAccess.Driver = "nfs"
		}
		if disk.HostAccess.MountPoint == "" {
			disk.HostAccess.MountPoint = filepath.Join("/Volumes/Colima", disk.Name)
		}
		if _, err := physicalDiskNFSSourcePath(disk.MountPoint); err != nil {
			return physicalDiskRuntime{}, err
		}
	}

	if disk.Backend != "nbd" && disk.Backend != "vz" {
		return physicalDiskRuntime{}, fmt.Errorf("unsupported physical disk backend %q", disk.Backend)
	}

	stateDir := filepath.Join(config.CurrentProfile().ConfigDir(), "physical-disks", disk.Name)
	return physicalDiskRuntime{
		PhysicalDisk: disk,
		index:        index,
		nbdDevice:    fmt.Sprintf("/dev/nbd%d", index),
		guestDevice:  physicalDiskGuestDevice(disk),
		nbdGuestPort: physicalDiskGuestNBDPortBase + index,
		stateDir:     stateDir,
	}, nil
}

func physicalDiskBackend(disk config.PhysicalDisk) string {
	if disk.Backend == "" || disk.Backend == "auto" {
		return "nbd"
	}
	return disk.Backend
}

func physicalDiskGuestDevice(disk config.PhysicalDisk) string {
	if physicalDiskBackend(disk) != "vz" {
		return ""
	}
	return filepath.Join("/dev/disk/by-id", "virtio-"+physicalDiskGuestBlockDeviceID(disk.Device))
}

func physicalDiskGuestBlockDeviceID(devicePath string) string {
	base := filepath.Base(devicePath)
	var b strings.Builder
	b.Grow(len(base))
	for _, r := range base {
		switch {
		case r >= 'a' && r <= 'z':
			b.WriteRune(r)
		case r >= 'A' && r <= 'Z':
			b.WriteRune(r)
		case r >= '0' && r <= '9':
			b.WriteRune(r)
		case r == '-', r == '_', r == '.':
			b.WriteRune(r)
		default:
			b.WriteByte('-')
		}
	}
	id := b.String()
	if id == "" {
		id = "block-device"
	}
	if len(id) > 20 {
		id = id[:20]
	}
	return id
}

func (l *limaVM) assertPhysicalDiskReady(disk physicalDiskRuntime) error {
	hostDevice := disk.RawDevice
	hostDeviceField := "rawDevice"
	if disk.Backend == "vz" {
		hostDevice = disk.Device
		hostDeviceField = "device"
	}
	if _, err := os.Stat(hostDevice); err != nil {
		return fmt.Errorf("%s %s is not accessible: %w", hostDeviceField, hostDevice, err)
	}
	out, err := l.host.RunOutput("diskutil", "info", disk.Device)
	if err != nil {
		return err
	}
	if disk.Writable && diskutilMounted(out) {
		return fmt.Errorf("%s is mounted on macOS; unmount it before using writable physicalDisks", disk.Device)
	}
	return nil
}

func diskutilMounted(out string) bool {
	for _, line := range strings.Split(out, "\n") {
		line = strings.TrimSpace(line)
		if strings.HasPrefix(line, "Mounted:") {
			return strings.Contains(line, "Yes")
		}
	}
	return false
}

func (l *limaVM) startPhysicalDiskNBD(disk physicalDiskRuntime) error {
	if err := os.MkdirAll(disk.stateDir, 0755); err != nil {
		return err
	}
	qemuNBD, err := exec.LookPath("qemu-nbd")
	if err != nil {
		return err
	}
	port, err := freeTCPPort()
	if err != nil {
		return err
	}
	disk.nbdHostPort = port
	if err := os.WriteFile(disk.nbdHostPortFile(), []byte(fmt.Sprint(port)), 0644); err != nil {
		return err
	}

	args := []string{
		qemuNBD,
		"--fork",
		"--persistent",
		"--format=raw",
		"--cache=none",
		"--bind=127.0.0.1",
		fmt.Sprintf("--port=%d", port),
		"--pid-file", disk.qemuPidFile(),
		disk.RawDevice,
	}

	if !disk.Writable {
		args = append(args[:1], append([]string{"--read-only"}, args[1:]...)...)
	}

	logFile := disk.qemuLogFile()
	cmd := strings.Join([]string{
		"set -eu",
		fmt.Sprintf("raw_device=%s", shellQuote(disk.RawDevice)),
		fmt.Sprintf("qemu_nbd=%s", shellQuote(qemuNBD)),
		"ps axww -o pid= -o command= | awk -v raw=\"$raw_device\" -v qemu=\"$qemu_nbd\" '{ pid=$1; $1=\"\"; sub(/^ +/, \"\"); if (index($0, qemu) == 1 && index($0, raw)) print pid }' | while read -r pid; do kill \"$pid\" >/dev/null 2>&1 || true; done",
		shellJoin(args) + " >" + shellQuote(logFile) + " 2>&1",
		fmt.Sprintf("chown %d:%d %s %s >/dev/null 2>&1 || true", os.Getuid(), os.Getgid(), shellQuote(disk.qemuPidFile()), shellQuote(logFile)),
		"chmod 0644 " + shellQuote(disk.qemuPidFile()) + " " + shellQuote(logFile) + " >/dev/null 2>&1 || true",
	}, "\n")
	if err := l.host.RunInteractive("sudo", "sh", "-c", cmd); err != nil {
		return appendPhysicalDiskLog(err, logFile, "qemu-nbd")
	}
	if err := l.waitTCPListen(port); err != nil {
		return appendPhysicalDiskLog(err, logFile, "qemu-nbd")
	}
	return nil
}

func (l *limaVM) startPhysicalDiskNBDTunnel(disk physicalDiskRuntime) error {
	port, err := readPort(disk.nbdHostPortFile())
	if err != nil {
		return err
	}
	disk.nbdHostPort = port

	target := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", disk.nbdGuestPort, disk.nbdHostPort)
	if err := l.startSSHForward(disk.nbdTunnelPidFile(), disk.nbdTunnelLogFile(), "-R", target); err != nil {
		return err
	}
	if err := l.waitHostPID(disk.nbdTunnelPidFile()); err != nil {
		return appendPhysicalDiskLog(err, disk.nbdTunnelLogFile(), "NBD SSH tunnel")
	}
	return nil
}

func (l *limaVM) attachPhysicalDiskInGuest(disk physicalDiskRuntime, guestDevice string) error {
	writable := "false"
	if disk.Writable {
		writable = "true"
	}
	return l.runGuestWith(strings.NewReader(physicalDiskScript),
		"sudo", "sh", "-s", "--",
		disk.Name,
		disk.Backend,
		guestDevice,
		fmt.Sprint(disk.nbdGuestPort),
		disk.MountPoint,
		disk.FSType,
		writable,
		fmt.Sprint(disk.HostAccess.Enabled),
	)
}

func (l *limaVM) startPhysicalDiskNFSTunnel(disk physicalDiskRuntime) error {
	port, err := freeTCPPort()
	if err != nil {
		return err
	}
	disk.nfsHostPort = port
	if err := os.WriteFile(disk.nfsHostPortFile(), []byte(fmt.Sprint(port)), 0644); err != nil {
		return err
	}

	target := fmt.Sprintf("127.0.0.1:%d:127.0.0.1:%d", port, physicalDiskNFSPort)
	if err := l.startSSHForward(disk.nfsTunnelPidFile(), disk.nfsTunnelLogFile(), "-L", target); err != nil {
		return err
	}
	if err := l.waitHostPID(disk.nfsTunnelPidFile()); err != nil {
		return appendPhysicalDiskLog(err, disk.nfsTunnelLogFile(), "NFS SSH tunnel")
	}
	if err := l.waitTCPListen(port); err != nil {
		return appendPhysicalDiskLog(err, disk.nfsTunnelLogFile(), "NFS SSH tunnel")
	}
	return nil
}

func (l *limaVM) mountPhysicalDiskOnHost(disk physicalDiskRuntime) error {
	port, err := readPort(disk.nfsHostPortFile())
	if err != nil {
		return err
	}
	disk.nfsHostPort = port

	if err := os.MkdirAll(disk.HostAccess.MountPoint, 0755); err != nil {
		if err := l.host.RunInteractive("sudo", "mkdir", "-p", disk.HostAccess.MountPoint); err != nil {
			return err
		}
	}
	if hostMountpointMounted(l, disk.HostAccess.MountPoint) {
		return nil
	}

	sourcePath, err := disk.nfsSourcePath()
	if err != nil {
		return err
	}

	opts := fmt.Sprintf("vers=4,tcp,nocallback,sec=sys,resvport,port=%d", disk.nfsHostPort)
	source := "localhost:" + sourcePath
	var lastErr error
	for i := 0; i < 10; i++ {
		if lastErr = l.host.RunInteractive("sudo", "mount", "-t", "nfs", "-o", opts, source, disk.HostAccess.MountPoint); lastErr == nil {
			return nil
		}
		time.Sleep(time.Second)
	}
	return lastErr
}

func (l *limaVM) stopPhysicalDisk(ctx context.Context, disk physicalDiskRuntime) error {
	var errs []string

	if disk.HostAccess.Enabled {
		if err := l.unmountPhysicalDiskOnHost(disk); err != nil {
			errs = append(errs, err.Error())
		}
	}
	if err := l.killHostPID(disk.nfsTunnelPidFile(), false); err != nil {
		errs = append(errs, err.Error())
	}

	if l.Running(ctx) {
		if err := l.detachPhysicalDiskInGuest(disk); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if disk.Backend == "nbd" {
		if err := l.killHostPID(disk.nbdTunnelPidFile(), false); err != nil {
			errs = append(errs, err.Error())
		}
		if err := l.killPhysicalDiskNBD(disk); err != nil {
			errs = append(errs, err.Error())
		}
	}

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "; "))
	}
	return nil
}

func (l *limaVM) detachPhysicalDiskInGuest(disk physicalDiskRuntime) error {
	lines := []string{
		"set -eu",
		fmt.Sprintf("export_file=%s", shellQuote(filepath.Join("/etc/exports.d", "colima-physical-"+disk.Name+".exports"))),
		fmt.Sprintf("export_root_file=%s", shellQuote(filepath.Join("/etc/exports.d", "colima-physical-root.exports"))),
		fmt.Sprintf("mount_point=%s", shellQuote(disk.MountPoint)),
		"rm -f \"$export_file\"",
		"if ! find /etc/exports.d -name 'colima-physical-*.exports' ! -name 'colima-physical-root.exports' | grep -q .; then rm -f \"$export_root_file\"; fi",
		"exportfs -ra >/dev/null 2>&1 || true",
		"if findmnt --noheadings --mountpoint \"$mount_point\" >/dev/null 2>&1; then umount \"$mount_point\" >/dev/null 2>&1 || true; fi",
	}
	if disk.Backend == "nbd" {
		lines = append(lines,
			fmt.Sprintf("nbd_device=%s", shellQuote(disk.nbdDevice)),
			"if command -v nbd-client >/dev/null 2>&1 && nbd-client -c \"$nbd_device\" >/dev/null 2>&1; then nbd-client -d \"$nbd_device\" >/dev/null 2>&1 || true; fi",
		)
	}
	script := strings.Join(lines, "\n")
	return l.runGuestQuiet("sudo", "sh", "-c", script)
}

func (l *limaVM) unmountPhysicalDiskOnHost(disk physicalDiskRuntime) error {
	if !hostMountpointMounted(l, disk.HostAccess.MountPoint) {
		return nil
	}
	if err := l.host.RunQuiet("sudo", "-n", "umount", disk.HostAccess.MountPoint); err == nil {
		return nil
	}
	return l.host.RunInteractive("sudo", "umount", disk.HostAccess.MountPoint)
}

func hostMountpointMounted(l *limaVM, mountPoint string) bool {
	cmd := "mount | grep -F " + shellQuote(" on "+mountPoint+" ") + " >/dev/null"
	return l.host.RunQuiet("sh", "-c", cmd) == nil
}

func (l *limaVM) assertPhysicalDiskBackendLive(disk physicalDiskRuntime) error {
	if disk.Backend == "vz" {
		script := fmt.Sprintf("test -b %s && findmnt --noheadings --mountpoint %s >/dev/null", shellQuote(disk.guestDevice), shellQuote(disk.MountPoint))
		return l.runGuestQuiet("sh", "-c", script)
	}

	if err := l.waitPIDExists(disk.qemuPidFile()); err != nil {
		return appendPhysicalDiskLog(err, disk.qemuLogFile(), "qemu-nbd")
	}
	if err := l.waitHostPID(disk.nbdTunnelPidFile()); err != nil {
		return appendPhysicalDiskLog(err, disk.nbdTunnelLogFile(), "NBD SSH tunnel")
	}
	port, err := readPort(disk.nbdHostPortFile())
	if err != nil {
		return err
	}
	if err := l.waitTCPListen(port); err != nil {
		return appendPhysicalDiskLog(err, disk.qemuLogFile(), "qemu-nbd")
	}
	return nil
}

func (l *limaVM) assertPhysicalDiskHostAccessLive(disk physicalDiskRuntime) error {
	if err := l.waitHostPID(disk.nfsTunnelPidFile()); err != nil {
		return appendPhysicalDiskLog(err, disk.nfsTunnelLogFile(), "NFS SSH tunnel")
	}
	port, err := readPort(disk.nfsHostPortFile())
	if err != nil {
		return err
	}
	if err := l.waitTCPListen(port); err != nil {
		return appendPhysicalDiskLog(err, disk.nfsTunnelLogFile(), "NFS SSH tunnel")
	}
	if !hostMountpointMounted(l, disk.HostAccess.MountPoint) {
		return fmt.Errorf("%s is not mounted on host", disk.HostAccess.MountPoint)
	}
	return nil
}

func (l *limaVM) startSSHForward(pidFile, logFile string, forwardArgs ...string) error {
	if err := os.MkdirAll(filepath.Dir(pidFile), 0755); err != nil {
		return err
	}
	_ = os.Remove(pidFile)
	_ = os.Remove(logFile)
	sshConfig := filepath.Join(config.CurrentProfile().LimaInstanceDir(), "ssh.config")
	target := "lima-" + config.CurrentProfile().ID
	forwardTarget := ""
	if len(forwardArgs) > 0 {
		forwardTarget = forwardArgs[len(forwardArgs)-1]
	}
	args := []string{
		"ssh",
		"-F", sshConfig,
		"-o", "ControlMaster=no",
		"-o", "ControlPath=none",
		"-o", "ExitOnForwardFailure=yes",
		"-o", "ServerAliveInterval=15",
		"-o", "ServerAliveCountMax=2",
		"-f",
		"-N",
	}
	args = append(args, forwardArgs...)
	args = append(args, target)

	cmd := strings.Join([]string{
		shellJoin(args) + " >" + shellQuote(logFile) + " 2>&1",
		"ps axww -o pid= -o command= | awk -v fwd=" + shellQuote(forwardTarget) + " -v target=" + shellQuote(target) + " '{ pid=$1; $1=\"\"; sub(/^ +/, \"\"); if ($0 ~ /^([^ ]*\\/)?ssh[[:space:]]/ && index($0, fwd) && index($0, target)) {print pid; exit} }' > " + shellQuote(pidFile),
		"test -s " + shellQuote(pidFile),
	}, "\n")
	return l.host.RunQuiet("sh", "-c", cmd)
}

func (l *limaVM) waitHostPID(pidFile string) error {
	cmd := "test -s " + shellQuote(pidFile) + " && kill -0 $(cat " + shellQuote(pidFile) + ")"
	var err error
	for i := 0; i < 20; i++ {
		if err = l.host.RunQuiet("sh", "-c", cmd); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return err
}

func (l *limaVM) waitPIDExists(pidFile string) error {
	cmd := "test -s " + shellQuote(pidFile) + " && ps -p $(cat " + shellQuote(pidFile) + ") >/dev/null"
	var err error
	for i := 0; i < 20; i++ {
		if err = l.host.RunQuiet("sh", "-c", cmd); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return err
}

func (l *limaVM) killHostPID(pidFile string, sudo bool) error {
	if _, err := os.Stat(pidFile); err != nil {
		return nil
	}
	if sudo {
		cmd := "if [ -s " + shellQuote(pidFile) + " ]; then kill $(cat " + shellQuote(pidFile) + ") >/dev/null 2>&1 || true; fi; rm -f " + shellQuote(pidFile)
		return l.host.RunInteractive("sudo", "sh", "-c", cmd)
	}
	cmd := "if [ -s " + shellQuote(pidFile) + " ]; then kill $(cat " + shellQuote(pidFile) + ") >/dev/null 2>&1 || true; fi; rm -f " + shellQuote(pidFile)
	return l.host.RunQuiet("sh", "-c", cmd)
}

func (l *limaVM) killPhysicalDiskNBD(disk physicalDiskRuntime) error {
	port, _ := readPort(disk.nbdHostPortFile())
	script := strings.Join([]string{
		"set +e",
		fmt.Sprintf("pid_file=%s", shellQuote(disk.qemuPidFile())),
		fmt.Sprintf("port=%s", shellQuote(fmt.Sprint(port))),
		"pids=",
		"if [ -s \"$pid_file\" ]; then pids=\"$pids $(cat \"$pid_file\")\"; fi",
		"if [ \"$port\" != \"0\" ] && command -v lsof >/dev/null 2>&1; then",
		"  pids=\"$pids $(lsof -tiTCP:\"$port\" -sTCP:LISTEN 2>/dev/null)\"",
		"fi",
		"if [ -n \"$pids\" ]; then kill $pids >/dev/null 2>&1 || true; fi",
		"alive=",
		"for pid in $pids; do ps -p \"$pid\" >/dev/null 2>&1 && alive=\"$alive $pid\"; done",
		"if [ -n \"$alive\" ]; then sudo -n kill $alive >/dev/null 2>&1 || true; fi",
		"rm -f \"$pid_file\"",
	}, "\n")
	if err := l.host.RunQuiet("sh", "-c", script); err != nil {
		return err
	}
	if !l.physicalDiskNBDRunning(disk) {
		return nil
	}

	qemuNBD, err := exec.LookPath("qemu-nbd")
	if err != nil {
		qemuNBD = "qemu-nbd"
	}
	rootScript := strings.Join([]string{
		"set +e",
		fmt.Sprintf("raw_device=%s", shellQuote(disk.RawDevice)),
		fmt.Sprintf("qemu_nbd=%s", shellQuote(qemuNBD)),
		fmt.Sprintf("pid_file=%s", shellQuote(disk.qemuPidFile())),
		"if [ -s \"$pid_file\" ]; then kill $(cat \"$pid_file\") >/dev/null 2>&1 || true; fi",
		"ps axww -o pid= -o command= | awk -v raw=\"$raw_device\" -v qemu=\"$qemu_nbd\" '{ pid=$1; $1=\"\"; sub(/^ +/, \"\"); if (index($0, qemu) == 1 && index($0, raw)) print pid }' | while read -r pid; do kill \"$pid\" >/dev/null 2>&1 || true; done",
		"rm -f \"$pid_file\"",
	}, "\n")
	if err := l.host.RunInteractive("sudo", "sh", "-c", rootScript); err != nil {
		return err
	}
	if l.physicalDiskNBDRunning(disk) {
		return fmt.Errorf("qemu-nbd for %s is still running", disk.RawDevice)
	}
	return nil
}

func (l *limaVM) runGuestQuiet(args ...string) error {
	args = append([]string{limactl, "shell", "--workdir", "/", config.CurrentProfile().ID}, args...)
	return l.host.RunQuiet(args...)
}

func (l *limaVM) runGuestWith(stdin io.Reader, args ...string) error {
	args = append([]string{limactl, "shell", "--workdir", "/", config.CurrentProfile().ID}, args...)
	return l.host.RunWith(stdin, nil, args...)
}

func freeTCPPort() (int, error) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, err
	}
	defer ln.Close()

	addr, ok := ln.Addr().(*net.TCPAddr)
	if !ok {
		return 0, fmt.Errorf("unexpected tcp listener address: %s", ln.Addr())
	}
	return addr.Port, nil
}

func readPort(file string) (int, error) {
	b, err := os.ReadFile(file)
	if err != nil {
		return 0, err
	}
	var port int
	if _, err := fmt.Sscanf(strings.TrimSpace(string(b)), "%d", &port); err != nil {
		return 0, err
	}
	return port, nil
}

func (p physicalDiskRuntime) nfsSourcePath() (string, error) {
	return physicalDiskNFSSourcePath(p.MountPoint)
}

func physicalDiskNFSSourcePath(mountPoint string) (string, error) {
	rel, err := filepath.Rel(physicalDiskNFSRoot, mountPoint)
	if err != nil {
		return "", err
	}
	if rel == "." || rel == ".." || strings.HasPrefix(rel, "../") || filepath.IsAbs(rel) {
		return "", fmt.Errorf("NFS host access requires mountPoint under %s", physicalDiskNFSRoot)
	}
	return "/" + filepath.ToSlash(rel), nil
}

func (l *limaVM) waitTCPListen(port int) error {
	var err error
	for i := 0; i < 40; i++ {
		if err = l.host.RunQuiet("sh", "-c", tcpListenCmd(port)); err == nil {
			return nil
		}
		time.Sleep(250 * time.Millisecond)
	}
	return err
}

func (l *limaVM) physicalDiskNBDRunning(disk physicalDiskRuntime) bool {
	if l.host.RunQuiet("sh", "-c", "test -s "+shellQuote(disk.qemuPidFile())+" && ps -p $(cat "+shellQuote(disk.qemuPidFile())+") >/dev/null") == nil {
		return true
	}
	port, err := readPort(disk.nbdHostPortFile())
	if err != nil || port == 0 {
		return false
	}
	return l.host.RunQuiet("sh", "-c", tcpListenCmd(port)) == nil
}

func tcpListenCmd(port int) string {
	return fmt.Sprintf("netstat -an -p tcp | awk '$4 ~ /[.:]%d$/ && $6 == \"LISTEN\" {found=1} END{exit !found}'", port)
}

func appendPhysicalDiskLog(err error, logFile, name string) error {
	b, readErr := os.ReadFile(logFile)
	if readErr != nil {
		return err
	}

	log := strings.TrimSpace(string(b))
	if log == "" {
		return err
	}
	const maxLogLen = 4096
	if len(log) > maxLogLen {
		log = log[:maxLogLen] + "\n..."
	}
	return fmt.Errorf("%w: %s log:\n%s", err, name, log)
}

func shellJoin(args []string) string {
	quoted := make([]string, len(args))
	for i, arg := range args {
		quoted[i] = shellQuote(arg)
	}
	return strings.Join(quoted, " ")
}

func shellQuote(s string) string {
	return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'"
}

func (p physicalDiskRuntime) qemuPidFile() string { return filepath.Join(p.stateDir, "qemu-nbd.pid") }
func (p physicalDiskRuntime) qemuLogFile() string { return filepath.Join(p.stateDir, "qemu-nbd.log") }
func (p physicalDiskRuntime) nbdTunnelPidFile() string {
	return filepath.Join(p.stateDir, "nbd-ssh.pid")
}
func (p physicalDiskRuntime) nbdTunnelLogFile() string {
	return filepath.Join(p.stateDir, "nbd-ssh.log")
}
func (p physicalDiskRuntime) nfsTunnelPidFile() string {
	return filepath.Join(p.stateDir, "nfs-ssh.pid")
}
func (p physicalDiskRuntime) nfsTunnelLogFile() string {
	return filepath.Join(p.stateDir, "nfs-ssh.log")
}
func (p physicalDiskRuntime) nbdHostPortFile() string {
	return filepath.Join(p.stateDir, "nbd-host.port")
}
func (p physicalDiskRuntime) nfsHostPortFile() string {
	return filepath.Join(p.stateDir, "nfs-host.port")
}
