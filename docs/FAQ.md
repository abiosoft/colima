# FAQs

- [FAQs](#faqs)
  - [How does Colima compare to Lima?](#how-does-colima-compare-to-lima)
  - [Are Apple Silicon Macs supported?](#are-apple-silicon-macs-supported)
  - [Does Colima support autostart?](#does-colima-support-autostart)
  - [Can config file be used instead of cli flags?](#can-config-file-be-used-instead-of-cli-flags)
    - [Specifying the config location](#specifying-the-config-location)
    - [Editing the config](#editing-the-config)
    - [Setting the default config](#setting-the-default-config)
    - [Specifying the config editor](#specifying-the-config-editor)
  - [Docker](#docker)
    - [Can it run alongside Docker for Mac?](#can-it-run-alongside-docker-for-mac)
    - [Docker socket location](#docker-socket-location)
      - [v0.3.4 or older](#v034-or-older)
      - [v0.4.0 or newer](#v040-or-newer)
      - [Listing Docker contexts](#listing-docker-contexts)
      - [Changing the active Docker context](#changing-the-active-docker-context)
    - [Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?](#cannot-connect-to-the-docker-daemon-at-unixvarrundockersock-is-the-docker-daemon-running)
    - [How to customize Docker config (e.g., adding insecure registries or registry mirrors)?](#how-to-customize-docker-config-eg-adding-insecure-registries-or-registry-mirrors)
    - [Docker buildx plugin is missing](#docker-buildx-plugin-is-missing)
      - [Installing Buildx](#installing-buildx)
  - [How does Colima compare to minikube, Kind, K3d?](#how-does-colima-compare-to-minikube-kind-k3d)
    - [For Kubernetes](#for-kubernetes)
    - [For Docker](#for-docker)
  - [Is another Distro supported?](#is-another-distro-supported)
    - [Version v0.5.6 and lower](#version-v056-and-lower)
      - [Enabling Ubuntu layer](#enabling-ubuntu-layer)
      - [Accessing the underlying Virtual Machine](#accessing-the-underlying-virtual-machine)
    - [Version v0.6.0 and newer](#version-v060-and-newer)
  - [The Virtual Machine's IP is not reachable](#the-virtual-machines-ip-is-not-reachable)
    - [Enable reachable IP address](#enable-reachable-ip-address)
  - [How can disk space be recovered?](#how-can-disk-space-be-recovered)
    - [Automatic](#automatic)
    - [Manual](#manual)
  - [How can disk size be increased?](#how-can-disk-size-be-increased)
  - [Are Lima overrides supported?](#are-lima-overrides-supported)
  - [Troubleshooting](#troubleshooting)
    - [Colima not starting](#colima-not-starting)
      - [Broken status](#broken-status)
      - [FATA\[0000\] error starting vm: error at 'starting': exit status 1](#fata0000-error-starting-vm-error-at-starting-exit-status-1)
    - [Issues after an upgrade](#issues-after-an-upgrade)
    - [Colima cannot access the internet.](#colima-cannot-access-the-internet)
    - [Docker Compose and Buildx showing runc error](#docker-compose-and-buildx-showing-runc-error)
      - [Version v0.5.6 or lower](#version-v056-or-lower)
    - [Issue with Docker bind mount showing empty](#issue-with-docker-bind-mount-showing-empty)
  - [How can Docker version be updated?](#how-can-docker-version-be-updated)

## How does Colima compare to Lima?

Colima is basically a higher level usage of Lima and utilises Lima to provide Docker, Containerd and/or Kubernetes.

## Are Apple Silicon Macs supported?

Colima supports and works on both Intel and Apple Silicon Macs.

Feedbacks would be appreciated.

## Does Colima support autostart?

Since v0.5.6 Colima supports foreground mode via the `--foreground` flag. i.e. `colima start --foreground`.

If Colima has been installed using brew, the easiest way to autostart Colima is to use brew services.

```sh
brew services start colima
```

## Can config file be used instead of cli flags?

Yes, from v0.4.0, Colima support YAML configuration file.

### Specifying the config location

Set the `$COLIMA_HOME` environment variable, otherwise it defaults to `$HOME/.colima`.

### Editing the config

```
colima start --edit
```

For manual edit, the config file is located at `$COLIMA_HOME/default/colima.yaml`.

For other profiles, `$COLIMA_HOME/<profile-name>/colima.yaml`

### Setting the default config

```
colima template
```

For manual edit, the template file is located at `$COLIMA_HOME/_templates/default.yaml`.

### Specifying the config editor

Set the `$EDITOR` environment variable or use the `--editor` flag.

```sh
colima start --edit --editor code # one-off config
colima template --editor code # default config
```

## Docker

### Can it run alongside Docker for Mac?

Yes, from version v0.3.0 Colima leverages Docker contexts and can thereby run alongside Docker for Mac.

Colima makes itself the default Docker context on startup and should work straight away.

### Docker socket location

#### v0.3.4 or older

Docker socket is located at `$COLIMA_HOME/docker.sock`

#### v0.4.0 or newer

Docker socket is located at `$COLIMA_HOME/default/docker.sock`

It can also be retrieved by checking status

```
colima status
```

#### Listing Docker contexts

```
docker context list
```

#### Changing the active Docker context

```
docker context use <context-name>
```
### Cannot connect to the Docker daemon at unix:///var/run/docker.sock. Is the docker daemon running?

Colima uses Docker contexts to allow co-existence with other Docker servers and sets itself as the default Docker context on startup.

However, some applications are not aware of Docker contexts and may lead to the error.

This can be fixed by any of the following approaches. Ensure the Docker socket path by checking the [socket location](#docker-socket-location).

1. Setting application specific Docker socket path if supported by the application. e.g. JetBrains IDEs.

2. Setting the `DOCKER_HOST` environment variable to point to Colima socket.

   ```sh
   export DOCKER_HOST="unix://${COLIMA_HOME}/default/docker.sock"
   ```
3. Linking the Colima socket to the default socket path. **Note** that this may break other Docker servers.

   ```sh
   sudo ln -sf $COLIMA_HOME/default/docker.sock /var/run/docker.sock
   ```


### How to customize Docker config (e.g., adding insecure registries or registry mirrors)?

* v0.3.4 or lower

  On first startup, Colima generates Docker daemon.json file at `$COLIMA_HOME/docker/daemon.json`.
  Modify the daemon.json file accordingly and restart Colima.

* v0.4.0 or newer

  Start Colima with `--edit` flag.

  ```sh
  colima start --edit
  ```

  Add the Docker config to the `docker` section.

  ```diff
  - docker: {}
  + docker:
  +   insecure-registries:
  +     - myregistry.com:5000
  +     - host.docker.internal:5000
  ```
**Note:** In order for the Docker client to respect (at least some) configuration value changes, modification of the host ~/.docker/daemon.json file may also be required.

For example, if adding registry mirrors, modifications are needed as follows:

First, colima:

```sh
colima start --edit
```

```diff
- docker: {}
+ docker:
+   registry-mirrors:
+     - https://my.dockerhub.mirror.something
+     - https://my.quayio.mirror.something
```

As an alternative approach to the **colima start --edit**, make the changes via the **template** command (affecting the configuration for any new instances):

```sh
colima template
```

Then, the Docker ~/.docker/daemon.json file (as compared to the default):

```diff
- "experimental": false,
+ "experimental": false,
+ "registry-mirrors": [
+   "https://my.dockerhub.mirror.something",
+   "https://my.quayio.mirror.something"  
+ ]
```  

### Docker buildx plugin is missing

`buildx` can be installed as a Docker plugin

#### Installing Buildx

Using homebrew
```sh
brew install docker-buildx
# Follow the caveats mentioned in the install instructions:
# mkdir -p ~/.docker/cli-plugins
# ln -sfn $(which docker-buildx) ~/.docker/cli-plugins/docker-buildx
docker buildx version # verify installation
```
Alternatively
```sh
ARCH=amd64 # change to 'arm64' for m1
VERSION=v0.11.2
curl -LO https://github.com/docker/buildx/releases/download/${VERSION}/buildx-${VERSION}.darwin-${ARCH}
mkdir -p ~/.docker/cli-plugins
mv buildx-${VERSION}.darwin-${ARCH} ~/.docker/cli-plugins/docker-buildx
chmod +x ~/.docker/cli-plugins/docker-buildx
docker buildx version # verify installation
```

## How does Colima compare to minikube, Kind, K3d?

### For Kubernetes

Yes, you can create a Kubernetes cluster with minikube (with Docker driver), Kind or K3d instead of enabling Kubernetes
in Colima.

Those are better options if you need multiple clusters, or do not need Docker and Kubernetes to share the same images and runtime.

Colima with Docker runtime is fully compatible with Minikube (with Docker driver), Kind and K3d.

### For Docker

Minikube with Docker runtime can expose the cluster's Docker with `minikube docker-env`. But there are some caveats.

- Kubernetes is not optional, even if you only need Docker.

- All of minikube's free drivers for macOS fall-short in one of performance, port forwarding or volumes. While  port-forwarding and volumes are non-issue for Kubernetes, they can be a deal breaker for Docker-only use.

## Is another Distro supported?

### Version v0.5.6 and lower

Colima uses a lightweight Alpine image with bundled dependencies.
Therefore, user interaction with the Virtual Machine is expected to be minimal (if any).

However, Colima optionally provides Ubuntu container as a layer.


#### Enabling Ubuntu layer

* CLI
  ```
  colima start --layer=true
  ```

* Config
  ```diff
  - layer: false
  + layer: true
  ```

#### Accessing the underlying Virtual Machine

When the layer is enabled, the underlying Virtual Machine is abstracted and both the `ssh` and `ssh-config` commands routes to the layer.

The underlying Virtual Machine is still accessible by specifying `--layer=false` to the `ssh` and `ssh-config` commands, or by running `colima` in the SSH session.

### Version v0.6.0 and newer

Colima uses Ubuntu as the underlying image. Other distros are not supported.

## The Virtual Machine's IP is not reachable

Reachable IP address is not enabled by default due to root privilege and slower startup time.

### Enable reachable IP address

**NOTE:** this is only supported on macOS

* CLI
  ```
  colima start --network-address
  ```
* Config
  ```diff
  network:
  -  address: false
  +  address: true
  ```

## How can disk space be recovered?

Disk space can be freed in the VM by removing containers or running `docker system prune`.
However, it will not reflect on the host on Colima versions v0.4.x or lower.

### Automatic

For Colima v0.5.0 and above, unused disk space in the VM is released on startup. A restart would suffice.

### Manual

For Colima v0.5.0 and above, user can manually recover the disk space by running `sudo fstrim -a` in the VM.

```sh
# '-v' may be added for verbose output
colima ssh -- sudo fstrim -a
```

## How can disk size be increased?

Disk size is automatically increased on start up based on configuration in `colima.yaml`

```diff
- disk: 150
+ disk: 250
```

__Note:__ This feature is available from Version 0.5.3.


## Are Lima overrides supported?

Yes, however this should only be done by advanced users.

Overriding the image is not supported as Colima's image includes bundled dependencies that would be missing in the user specified image.

## Troubleshooting

These are some common issues reported by users and how to troubleshoot them.

### Colima not starting

There are multiple reasons that could cause Colima to fail to start.

#### Broken status

This is the case when the output of `colima list` shows a broken status. This can happen due to macOS restart.

```
colima list
PROFILE    STATUS     ARCH       CPUS    MEMORY    DISK     RUNTIME    ADDRESS
default    Broken     aarch64    2       2GiB      60GiB
```
This can be fixed by forcefully stopping Colima. The state will be changed to `Stopped` and it should start up normally afterwards.

```
colima stop --force
```

#### FATA[0000] error starting vm: error at 'starting': exit status 1

This indicates that a fatal error is preventing Colima from starting, you can enable the debug log with `--verbose` flag to get more info.

If the log output includes `exiting, status={Running:false Degraded:false Exiting:true Errors:[] SSHLocalPort:0}` then it is most certainly due to one of the following.

1. Running on a device without virtualization support.
2. Running an x86_64 version of homebrew (and Colima) on an M1 device.

### Issues after an upgrade

The recommended way to troubleshoot after an upgrade is to test with a separate profile.

```sh
# start with a profile named 'debug'
colima start debug
```
If the separate profile starts successfully without issues, then the issue would be resolved by resetting the default profile.

```
colima delete
colima start
```

### Colima cannot access the internet.

Failure for Colima to access the internet is usually down to DNS.

Try custom DNS server(s)

```sh
colima start --dns 8.8.8.8 --dns 1.1.1.1
```

Ping an internet address from within the VM to ascertain

```
colima ssh -- ping -c4 google.com
PING google.com (216.58.223.238): 56 data bytes
64 bytes from 216.58.223.238: seq=0 ttl=42 time=0.082 ms
64 bytes from 216.58.223.238: seq=1 ttl=42 time=0.557 ms
64 bytes from 216.58.223.238: seq=2 ttl=42 time=0.465 ms
64 bytes from 216.58.223.238: seq=3 ttl=42 time=0.457 ms

--- google.com ping statistics ---
4 packets transmitted, 4 packets received, 0% packet loss
round-trip min/avg/max = 0.082/0.390/0.557 ms
```

### Docker Compose and Buildx showing runc error

#### Version v0.5.6 or lower

Recent versions of Buildkit may show the following error.

```console
runc run failed: unable to start container process: error during container init: error mounting "cgroup" to rootfs at "/sys/fs/cgroup": mount cgroup:/sys/fs/cgroup/openrc (via /proc/self/fd/6), flags: 0xf, data: openrc: invalid argument
```

From v0.5.6, start Colima with `--cgroups-v2` flag as a workaround.

**This is fixed in v0.6.0.**

### Issue with Docker bind mount showing empty

When using docker to bind mount a volume (e.g. using `-v` or `--mount`) from the host where the volume is not contained within `/tmp/colima` or `/Users/$USER`, the container will start without raising any errors but the mapped mountpoint on the container will be empty.

This is rectified by mounting the volume on the VM, and only then can docker map the volume or any subdirectory. Edit `$COLIMA_HOME/default/colima.yaml` and add to the `mounts` section (examples are provided within the yaml file), and then run `colima restart`. Start the container again with the desired bind mount and it should show up correctly.

## How can Docker version be updated?

Each Colima release includes the latest Docker version at the time of release.

From v0.7.6, there is a new `colima update` command to update the container runtime without needing to update Colima or to wait for the next Colima release.
