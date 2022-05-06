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

//

//go:embed Dockerfile
var dockerfile string

//go:embed colima-run.sh
var runScript string

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

func (u ubuntuRuntime) imageBasename() string {
	return "ubuntu-layer-" + string(u.guest.Arch().Value())
}

func (u ubuntuRuntime) imageArchive() string {
	name := "ubuntu-layer-" + string(u.guest.Arch().Value()) + ".tar.gz"
	return filepath.Join("/usr/share/colima", name)
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
		if err := u.guest.Write(filepath.Join(tmpDir, "colima-run"), runScript); err != nil {
			return fmt.Errorf("error writing ubuntu layer chroot script: %w", err)
		}
		if err := u.guest.RunQuiet("sudo", "chown", "-R", b.username, tmpDir); err != nil {
			return fmt.Errorf("error preparing ubuntu layer cache dir: %w", err)
		}

		//sshAuthFile := filepath.Join(b.homeDir, ".ssh/authorized_keys")
		//sshAuthFileDest := filepath.Join(tmpDir, "authorized_keys")
		//if err := u.guest.RunQuiet("cp", sshAuthFile, sshAuthFileDest); err != nil {
		//	return fmt.Errorf("error writing ubuntu layer dockerfile: %w", err)
		//}

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
	hostname := config.Profile().ID
	args := nerdctl("create",
		"--name", containerName,
		"--hostname", hostname,
		"--add-host", hostname+":127.0.0.1",
		"--privileged",
		"--net", "host",
		"--pid", "host",
		"--volume", "/home/"+username+".linux:/home/"+username,
		"--volume", "/var/run/docker.sock:/var/run/docker.sock:ro",
		"--volume", "/:/host",
	)

	mounts := conf.Mounts
	if len(mounts) == 0 {
		// TODO: should be not be repeated here but rather populated externally
		mounts = append(mounts, config.Mount{Location: util.HomeDir()})
		mounts = append(mounts, config.Mount{Location: filepath.Join("/tmp", config.Profile().ID)})
	}
	for _, m := range mounts {
		args = append(args, "--volume", m.Location+":"+m.Location)
	}

	env := conf.Env
	for k, v := range env {
		args = append(args, "--env", k+"="+v)
	}

	args = append(args, imageName)
	return u.guest.Run(args...)
}
