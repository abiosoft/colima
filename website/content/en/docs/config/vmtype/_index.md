---
title: VM types
weight: 10
---

Lima supports several VM drivers for running guest machines:

The vmType can be specified only on creating the instance.
The vmType of existing instances cannot be changed.

> **💡 For developers**: See [Virtual Machine Drivers](../../dev/drivers) for technical details about driver architecture and creating custom drivers.


See the following flowchart to choose the best vmType for you:
```mermaid
flowchart
  host{"Host OS"} -- "Windows" --> wsl2["WSL2"]
  host -- "Linux" --> qemu["QEMU"]
  host -- "macOS" --> intel_on_arm{"Need to run <br> Intel binaries <br> on ARM?"}
  intel_on_arm -- "Yes" --> just_elf{"Just need to <br> run Intel userspace (fast), <br> or entire Intel VM (slow)?"}
  just_elf -- "Userspace (fast)" --> vz
  just_elf -- "VM (slow)" --> qemu
  intel_on_arm --  "No" --> vz["VZ"]
```

The default vmType is QEMU in Lima prior to v1.0.
Starting with Lima v1.0, Lima will use VZ by default on macOS (>= 13.5) for new instances,
unless the config is incompatible with VZ. (e.g., legacyBIOS or 9p is enabled)