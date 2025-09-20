---
title: user-v2 network
weight: 32
---

| ⚡ Requirement | Lima >= 0.16.0 |
|-------------------|----------------|

user-v2 network provides a user-mode networking similar to the [default user-mode network](#user-mode-network--1921685024-) and also provides support for `vm -> vm` communication.

To enable this network mode, define a network with `mode: user-v2` in networks.yaml

By default, the below network configuration is already applied (Since v0.18).

```yaml
...
networks:
  user-v2:
    mode: user-v2
    gateway: 192.168.104.1
    netmask: 255.255.255.0
...
```

Instances can then reference these networks from their `lima.yaml` file:

{{< tabpane text=true >}}
{{% tab header="CLI" %}}
```bash
limactl start --network=lima:user-v2
```
{{% /tab %}}
{{% tab header="YAML" %}}
```yaml
networks:
   - lima: user-v2
```
{{% /tab %}}
{{< /tabpane >}}

An instance's IP address is resolvable from another instance as `lima-<NAME>.internal.` (e.g., `lima-default.internal.`).

_Note_

- Enabling this network will disable the [default user-mode network]({{< ref "/docs/config/network/user" >}})
