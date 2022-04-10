package cmd

import (
	"bytes"
	"fmt"
	"os"
	"os/exec"
	"runtime"
	"strings"
	"time"

	"github.com/abiosoft/colima/cli"
	"github.com/abiosoft/colima/cmd/root"
	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/config/configmanager"
	"github.com/abiosoft/colima/embedded"
	"github.com/abiosoft/colima/environment"
	"github.com/abiosoft/colima/environment/container/docker"
	"github.com/abiosoft/colima/environment/container/kubernetes"
	"github.com/abiosoft/colima/util"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
)

// startCmd represents the start command
var startCmd = &cobra.Command{
	Use:   "start [profile]",
	Short: "start Colima",
	Long: `Start Colima with the specified container runtime and optional kubernetes.
To customize with a more expressive configuration file, start with --edit flag.
`,
	Example: "  colima start\n" +
		"  colima start --edit\n" +
		"  colima start --runtime containerd\n" +
		"  colima start --with-kubernetes\n" +
		"  colima start --runtime containerd --with-kubernetes\n" +
		"  colima start --cpu 4 --memory 8 --disk 100\n" +
		"  colima start --arch aarch64\n" +
		"  colima start --dns 1.1.1.1 --dns 8.8.8.8",
	Args: cobra.MaximumNArgs(1),
	RunE: func(cmd *cobra.Command, args []string) error {
		app := newApp()
		conf := startCmdArgs.Config

		if !startCmdArgs.Flags.Edit {
			return app.Start(conf)
		}

		// edit flag is specified
		err := editConfigFile()
		if err != nil {
			return err
		}

		conf, err = configmanager.Load()
		if err != nil {
			return fmt.Errorf("error opening config file: %w", err)
		}

		if app.Active() {
			log.Println("colima is currently running, shutting down to apply changes")
			if err := app.Stop(false); err != nil {
				return fmt.Errorf("error stopping :%w", err)
			}
			// pause before startup to prevent race condition
			time.Sleep(time.Second * 5)
		}

		return app.Start(conf)
	},
	PreRunE: func(cmd *cobra.Command, args []string) error {
		// combine args and current config file(if any)
		prepareConfig(cmd)

		// persist in preparing of application start
		if err := configmanager.Save(startCmdArgs.Config); err != nil {
			return fmt.Errorf("error preparing config file: %w", err)
		}

		return nil
	},
}

const (
	defaultCPU               = 2
	defaultMemory            = 2
	defaultDisk              = 60
	defaultKubernetesVersion = kubernetes.DefaultVersion
)

var startCmdArgs struct {
	config.Config

	Flags struct {
		Mounts           []string
		LegacyKubernetes bool // for backward compatibility
		Edit             bool
		Editor           string
	}
}

func init() {
	runtimes := strings.Join(environment.ContainerRuntimes(), ", ")
	defaultArch := string(environment.Arch(runtime.GOARCH).Value())

	root.Cmd().AddCommand(startCmd)
	startCmd.Flags().StringVarP(&startCmdArgs.Runtime, "runtime", "r", docker.Name, "container runtime ("+runtimes+")")
	startCmd.Flags().IntVarP(&startCmdArgs.CPU, "cpu", "c", defaultCPU, "number of CPUs")
	startCmd.Flags().StringVar(&startCmdArgs.CPUType, "cpu-type", "", "the CPU type, options can be checked with 'qemu-system-"+defaultArch+" -cpu help'")
	startCmd.Flags().IntVarP(&startCmdArgs.Memory, "memory", "m", defaultMemory, "memory in GiB")
	startCmd.Flags().IntVarP(&startCmdArgs.Disk, "disk", "d", defaultDisk, "disk size in GiB")
	startCmd.Flags().StringVarP(&startCmdArgs.Arch, "arch", "a", defaultArch, "architecture (aarch64, x86_64)")

	// network
	if util.MacOS() {
		startCmd.Flags().BoolVar(&startCmdArgs.Network.Address, "network-address", true, "assign reachable IP address to the VM")
		startCmd.Flags().BoolVar(&startCmdArgs.Network.UserMode, "network-user-mode", false, "use Qemu user-mode network for internet, always true if --network-address=false")
	}

	// config
	startCmd.Flags().BoolVarP(&startCmdArgs.Flags.Edit, "edit", "e", false, "edit the configuration file before starting")
	startCmd.Flags().StringVar(&startCmdArgs.Flags.Editor, "editor", "", `editor to use for edit e.g. vim, nano, code (default "$EDITOR" env var)`)

	// mounts
	startCmd.Flags().StringSliceVarP(&startCmdArgs.Flags.Mounts, "mount", "V", nil, "directories to mount, suffix ':w' for writable")
	startCmd.Flags().StringVar(&startCmdArgs.MountType, "mount-type", "9p", "volume driver for the mount (9p, reverse-sshfs)")

	// ssh agent
	startCmd.Flags().BoolVarP(&startCmdArgs.ForwardAgent, "ssh-agent", "s", false, "forward SSH agent to the VM")

	// k8s
	startCmd.Flags().BoolVarP(&startCmdArgs.Kubernetes.Enabled, "kubernetes", "k", false, "start VM with Kubernetes")
	startCmd.Flags().BoolVar(&startCmdArgs.Flags.LegacyKubernetes, "with-kubernetes", false, "start VM with Kubernetes")
	startCmd.Flags().StringVar(&startCmdArgs.Kubernetes.Version, "kubernetes-version", defaultKubernetesVersion, "must match a k3s version https://github.com/k3s-io/k3s/releases")
	startCmd.Flags().BoolVar(&startCmdArgs.Kubernetes.Ingress, "kubernetes-ingress", false, "enable traefik ingress controller")
	startCmd.Flag("with-kubernetes").Hidden = true

	// not sure of the usefulness of env vars for now considering that interactions will be with the containers, not the VM.
	// leaving it undocumented until there is a need.
	startCmd.Flags().StringToStringVar(&startCmdArgs.Env, "env", nil, "environment variables for the VM")

	startCmd.Flags().IPSliceVarP(&startCmdArgs.DNS, "dns", "n", nil, "DNS servers for the VM")
}

// mountsFromFlag converts mounts from cli flag format to config file format
func mountsFromFlag(mounts []string) []config.Mount {
	mnts := make([]config.Mount, len(mounts))
	for i, mount := range mounts {
		str := strings.SplitN(string(mount), ":", 2)
		mnts[i] = config.Mount{
			Location: str[0],
			Writable: len(str) >= 2 && str[1] == "w",
		}
	}
	return mnts
}

func prepareConfig(cmd *cobra.Command) {
	current, err := configmanager.Load()
	if err != nil {
		// not fatal, will proceed with defaults
		log.Warnln(fmt.Errorf("config load failed: %w", err))
		log.Warnln("reverting to default settings")
	}

	// handle legacy kubernetes flag
	if cmd.Flag("with-kubernetes").Changed {
		startCmdArgs.Kubernetes.Enabled = startCmdArgs.Flags.LegacyKubernetes
		cmd.Flag("kubernetes").Changed = true
	}

	// convert cli to config file format
	startCmdArgs.Mounts = mountsFromFlag(startCmdArgs.Flags.Mounts)

	// use default config
	if current.Empty() {
		return
	}

	// disk size, and arch are only effective on VM create
	// set it to the current settings
	startCmdArgs.Disk = current.Disk
	startCmdArgs.Arch = current.Arch
	startCmdArgs.Docker = current.Docker

	// use current settings for unchanged configs
	// otherwise may be reverted to their default values.
	if !cmd.Flag("kubernetes").Changed {
		startCmdArgs.Kubernetes.Enabled = current.Kubernetes.Enabled
	}
	if !cmd.Flag("runtime").Changed {
		startCmdArgs.Runtime = current.Runtime
	}
	if !cmd.Flag("cpu").Changed {
		startCmdArgs.CPU = current.CPU
	}
	if !cmd.Flag("cpu-type").Changed {
		startCmdArgs.CPUType = current.CPUType
	}
	if !cmd.Flag("memory").Changed {
		startCmdArgs.Memory = current.Memory
	}
	if !cmd.Flag("mount").Changed {
		startCmdArgs.Mounts = current.Mounts
	}
	if !cmd.Flag("mount-type").Changed {
		startCmdArgs.MountType = current.MountType
	}
	if !cmd.Flag("ssh-agent").Changed {
		startCmdArgs.ForwardAgent = current.ForwardAgent
	}
	if !cmd.Flag("dns").Changed {
		startCmdArgs.DNS = current.DNS
	}
	if util.MacOS() {
		if !cmd.Flag("network-address").Changed {
			startCmdArgs.Network.Address = current.Network.Address
		}
		if !cmd.Flag("network-user-mode").Changed {
			startCmdArgs.Network.UserMode = current.Network.UserMode
		}
	}
	// remaining settings do not survive VM reboots.
}

// editConfigFile launches an editor to edit the config file.
// It runs true if startup should continue, or false if the user
// terminates the process.
func editConfigFile() error {
	// preserve the current file in case the user terminates
	currentFile, err := os.ReadFile(config.File())
	if err != nil {
		return fmt.Errorf("error reading config file: %w", err)
	}

	// prepend the config file with termination instruction
	abort, err := embedded.ReadString("defaults/abort.yaml")
	if err != nil {
		log.Warnln(fmt.Errorf("unable to read embedded file: %w", err))
	}

	if err := os.WriteFile(config.File(), []byte(abort+"\n"+string(currentFile)), 0644); err != nil {
		return fmt.Errorf("error writing config file for edit: %w", err)
	}

	err = launchEditor(startCmdArgs.Flags.Editor, config.File())
	if err != nil {
		return fmt.Errorf("error editing config file: %w", err)
	}

	// if file is empty, abort
	if f, err := os.ReadFile(config.File()); err == nil && len(bytes.TrimSpace(f)) == 0 {
		// restore original contents
		if err := os.WriteFile(config.File(), currentFile, 0644); err != nil {
			log.Warnln("error restoring config file: %w", err)
		}
		return fmt.Errorf("empty file, startup aborted")
	}

	return nil
}

var editors = []string{
	"vim",
	"code --wait --new-window",
	"nano",
}

func launchEditor(editor string, file string) error {
	// if not specified, prefer vscode if this a vscode terminal
	if editor == "" {
		if os.Getenv("TERM_PROGRAM") == "vscode" {
			editor = "code --wait"
		}
	}

	// if not found, check the EDITOR env var
	if editor == "" {
		if e := os.Getenv("EDITOR"); e != "" {
			editor = e
		}
	}

	// if not found, check the preferred editors
	if editor == "" {
		for _, e := range editors {
			s := strings.Fields(e)
			if _, err := exec.LookPath(s[0]); err == nil {
				editor = e
				break
			}
		}
	}

	// if still not found, abort
	if editor == "" {
		return fmt.Errorf("no editor found in $PATH, kindly set $EDITOR environment variable and try again")
	}

	// vscode needs the wait flag, add it if the user did not.
	if editor == "code" {
		editor = "code --wait --new-window"
	}

	args := strings.Fields(editor)
	cmd := args[0]
	args = append(args[1:], file)

	return cli.CommandInteractive(cmd, args...).Run()
}
