---
title: Usage
weight: 2
---

## Starting Docker/Containerd

Run `colima start` to start the default Docker runtime.

```console
$ colima start
INFO[0000] starting colima
INFO[0000] runtime: docker
INFO[0029] provisioning ...
INFO[0029] starting ...
INFO[0029] done
```

For automation, additional flags can be used to customize the configuration:

```bash
# Start with custom CPU, memory, and disk
colima start --cpu 4 --memory 8 --disk 100

# Start with containerd runtime
colima start --runtime containerd

# Start with Kubernetes support
colima start --kubernetes
```

### Docker Usage
Once started, you can use Docker as normal:
```bash
docker run hello-world
docker ps
docker images
```

### Executing commands in the VM
Run `colima ssh` to open a shell session in the VM:
```bash
colima ssh
```

You can also execute specific commands:
```bash
colima ssh -- uname -a
```

### Kubernetes Usage
If started with `--kubernetes`, you can use kubectl:
```bash
kubectl get nodes
kubectl apply -f deployment.yaml
```

See also the command reference:
- [`colima start`](../reference/colima_start/)
- [`colima stop`](../reference/colima_stop/)
- [`colima status`](../reference/colima_status/)
- [`colima ssh`](../reference/colima_ssh/)

### Shell completion
- To enable bash completion, add `source <(colima completion bash)` to `~/.bash_profile`.
- To enable zsh completion, see `colima completion zsh --help`
