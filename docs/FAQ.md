# FAQs

- [FAQs](#faqs)
  - [How does Colima compare to Lima?](#how-does-colima-compare-to-lima)
  - [Are M1 macs supported?](#are-m1-macs-supported)
  - [Can config file be used instead of cli flags?](#can-config-file-be-used-instead-of-cli-flags)
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
    - [How to customize Docker config e.g. add insecure registries?](#how-to-customize-docker-config-eg-add-insecure-registries)
    - [Docker plugins are missing (buildx, scan)](#docker-plugins-are-missing-buildx-scan)
      - [Installing Buildx](#installing-buildx)
      - [Installing Docker Scan](#installing-docker-scan)
  - [How does Colima compare to minikube, Kind, K3d?](#how-does-colima-compare-to-minikube-kind-k3d)
    - [For Kubernetes](#for-kubernetes)
    - [For Docker](#for-docker)
  - [Is another Distro supported?](#is-another-distro-supported)
    - [Enabling Ubuntu layer](#enabling-ubuntu-layer)
    - [Accessing the underlying Virtual Machine](#accessing-the-underlying-virtual-machine)
  - [The Virtual Machine's IP is not reachable](#the-virtual-machines-ip-is-not-reachable)
    - [Enable reachable IP address](#enable-reachable-ip-address)
  - [Are Lima overrides supported?](#are-lima-overrides-supported)

## How does Colima compare to Lima?

Colima is basically a higher level usage of Lima and utilises Lima to provide Docker, Containerd and/or Kubernetes.

## Are M1 macs supported?

Colima supports and works on both Intel and M1 macs.

Feedbacks would be appreciated.

## Can config file be used instead of cli flags?

Yes, from v0.4.0, Colima support YAML configuration file.

### Editing the config

```
colima start --edit
```

For manual edit, the config file is located at `$HOME/.colima/default/colima.yaml`.

For other profiles, `$HOME/.colima/<profile-name>/colima.yaml`

### Setting the default config

```
colima template
```

For manual edit, the template file is located at `$HOME/.colima/_templates/default.yaml`.

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

Docker socket is located at `$HOME/.colima/docker.sock`

#### v0.4.0 or newer

Docker socket is located at `$HOME/.colima/default/docker.sock`

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
   export DOCKER_HOST="unix://${HOME}/.colima/default/docker.sock"
   ```
3. Linking the Colima socket to the default socket path. **Note** that this may break other Docker servers. 

   ```sh
   sudo ln -sf $HOME/.colima/default/docker.sock /var/run/docker.sock
   ```


### How to customize Docker config e.g. add insecure registries?

* v0.3.4 or lower

  On first startup, Colima generates Docker daemon.json file at `$HOME/.colima/docker/daemon.json`.
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

### Docker plugins are missing (buildx, scan)

Both buildx and scan can be installed as Docker plugins

#### Installing Buildx

```sh
ARCH=amd64 # change to 'arm64' for m1
VERSION=v0.8.2
curl -LO https://github.com/docker/buildx/releases/download/${VERSION}/buildx-${VERSION}.darwin-${ARCH}
mkdir -p ~/.docker/cli-plugins
mv buildx-${VERSION}.darwin-${ARCH} ~/.docker/cli-plugins/docker-buildx
chmod +x ~/.docker/cli-plugins/docker-buildx
docker buildx version # verify installation
```
#### Installing Docker Scan

Install Synk CLI

```sh
brew install snyk/tap/snyk
```

Install Docker Scan

```sh
ARCH=amd64 # change to 'arm64' for m1
VERSION=v0.17.0
curl -LO https://github.com/docker/scan-cli-plugin/releases/download/${VERSION}/docker-scan_darwin_${ARCH}
mkdir -p ~/.docker/cli-plugins
mv docker-scan_darwin_${ARCH} ~/.docker/cli-plugins/docker-scan
chmod +x ~/.docker/cli-plugins/docker-scan
docker scan --version # verify installation
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

Colima uses a lightweight Alpine image with bundled dependencies.
Therefore, user interaction with the Virtual Machine is expected to be minimal (if any).

However, Colima optionally provides Ubuntu container as a layer.

### Enabling Ubuntu layer

* CLI
  ```
  colima start --layer=true
  ```

* Config
  ```diff
  - layer: false
  + layer: true
  ```

### Accessing the underlying Virtual Machine

When the layer is enabled, the underlying Virtual Machine is abstracted and both the `ssh` and `ssh-config` commands routes to the layer.

The underlying Virtual Machine is still accessible by specifying `--layer=false` to the `ssh` and `ssh-config` commands, or by running `colima` in the SSH session.

## The Virtual Machine's IP is not reachable

This is by design. Reachable IP address is not enabled by default because it requires root access.

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

## Are Lima overrides supported?

Yes, however this should only be done by advanced users.

Overriding the image is not supported as Colima's image includes bundled dependencies that would be missing in the user specified image.
