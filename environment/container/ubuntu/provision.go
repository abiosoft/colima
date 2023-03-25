package ubuntu

import (
	_ "embed"
	"fmt"
	"path/filepath"
	"strconv"

	"github.com/abiosoft/colima/config"
	"github.com/abiosoft/colima/util"
)

type buildArgs struct {
	username  string
	homeDir   string
	arch      string
	uid       int
	gid       int
	dockerGid int
}

//go:embed Dockerfile
var dockerfile []byte

//go:embed colima.sh
var chrootScript []byte

func (u ubuntuRuntime) buildArgs() (b buildArgs, err error) {
	b.arch = string(u.guest.Arch().Value())

	if b.username, err = u.guest.User(); err != nil {
		return
	}

	if b.homeDir, err = u.guest.RunOutput("sh", "-c", "echo $HOME"); err != nil {
		return b, fmt.Errorf("error retrieving home dir: %w", err)
	}
	{
		uid, err := u.guest.RunOutput("id", "-u")
		if err != nil {
			return b, fmt.Errorf("error retrieving user id: %w", err)
		}
		b.uid, err = strconv.Atoi(uid)
		if err != nil {
			return b, fmt.Errorf("invalid user id: %v", uid)
		}
	}
	{
		gid, err := u.guest.RunOutput("id", "-g")
		if err != nil {
			return b, fmt.Errorf("error retrieving group id: %w", err)
		}
		b.gid, err = strconv.Atoi(gid)
		if err != nil {
			return b, fmt.Errorf("invalid group id: %v", gid)
		}
	}
	{
		gid, err := u.guest.RunOutput("sh", "-c", "getent group docker | cut -d: -f3")
		if err != nil {
			return b, fmt.Errorf("error retrieving docker group id: %w", err)
		}
		b.dockerGid, err = strconv.Atoi(gid)
		if err != nil {
			return b, fmt.Errorf("invalid docker group id: %v", gid)
		}
	}

	return
}

func (u ubuntuRuntime) imageCreated() bool {
	args := nerdctl("image", "inspect", imageName)
	return u.guest.RunQuiet(args...) == nil
}

func (u ubuntuRuntime) createImage() error {
	b, err := u.buildArgs()
	if err != nil {
		return fmt.Errorf("error getting image build args: %w", err)
	}

	// prerequisite
	{
		args := []string{"nerdctl", "--namespace", "buildkit", "load", "--input", imageArchive}
		if err := u.guest.Run(args...); err != nil {
			return fmt.Errorf("error loading ubuntu layer image: %w", err)
		}
	}
	defer func() {
		_ = u.guest.RunQuiet("nerdctl", "--namespace", "buildkit", "rmi", "--force", imageName+"-"+b.arch)
	}()

	// build dockerfile
	{

		const tmpDir = "/tmp/ubuntu"
		dockerfilePath := filepath.Join(tmpDir, "Dockerfile")

		if err := u.guest.Write(dockerfilePath, dockerfile); err != nil {
			return fmt.Errorf("error writing ubuntu layer dockerfile: %w", err)
		}
		if err := u.guest.Write(filepath.Join(tmpDir, "colima"), chrootScript); err != nil {
			return fmt.Errorf("error writing ubuntu layer chroot script: %w", err)
		}
		if err := u.guest.RunQuiet("sudo", "chown", "-R", b.username, tmpDir); err != nil {
			return fmt.Errorf("error preparing ubuntu layer cache dir: %w", err)
		}

		args := nerdctl(
			"build",
			"--tag", imageName,
			"--build-arg", "ARCH="+b.arch,
			"--build-arg", "NONROOT_USER="+b.username,
			"--build-arg", "UID="+strconv.Itoa(b.uid),
			"--build-arg", "GID="+strconv.Itoa(b.gid),
			"--build-arg", "DOCKER_GID="+strconv.Itoa(b.dockerGid),
			"--progress", "plain",
			"--file", dockerfilePath,
			tmpDir,
		)

		if err := u.guest.Run(args...); err != nil {
			return fmt.Errorf("error building ubuntu layer image: %w", err)
		}
	}

	return nil
}

func (u ubuntuRuntime) containerCreated() bool {
	args := nerdctl("container", "inspect", containerName)
	return u.guest.RunQuiet(args...) == nil
}

func (u ubuntuRuntime) createContainer(conf config.Config) error {
	username, err := u.guest.User()
	if err != nil {
		return fmt.Errorf("error retrieving username in guest: %w", err)
	}
	hostname := config.CurrentProfile().ID
	home := "/home/" + username + ".linux"
	args := nerdctl("create",

		// essentials
		"--name", containerName,
		"--hostname", hostname,
		"--privileged",
		"--net", "host",
		"--volume", home+":"+home,
		"--volume", "/:/host",

		// systemd
		"--mount", "type=bind,source=/sys/fs/cgroup,target=/sys/fs/cgroup",
		"--mount", "type=bind,source=/sys/fs/fuse,target=/sys/fs/fuse",
		"--mount", "type=bind,source=/tmp/ubuntu,destination=/tmp",
		"--mount", "type=tmpfs,destination=/run",
		"--mount", "type=tmpfs,destination=/run/lock",
	)

	// colima mounts
	mounts := conf.MountsOrDefault()
	for _, m := range mounts {
		location := m.MountPoint
		if location == "" {
			location = m.Location
		}

		location, err := util.CleanPath(location)
		if err != nil {
			return err
		}
		args = append(args, "--volume", location+":"+location)
	}

	// environment variables propagation
	env := conf.Env
	for k, v := range env {
		args = append(args, "--env", k+"="+v)
	}

	// image
	args = append(args, imageName)

	return u.guest.Run(args...)
}

func (u ubuntuRuntime) syncHostname() error {
	currentHostname := func() string {
		args := nerdctl("exec", containerName, "hostname")
		hostname, _ := u.guest.RunOutput(args...)
		return hostname
	}()

	hostname := config.CurrentProfile().ID
	if currentHostname == hostname {
		return nil
	}

	args := nerdctl("exec", containerName, "sudo", "hostname", hostname)
	return u.guest.RunQuiet(args...)
}
