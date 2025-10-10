---
title: VM types
weight: 10
---

Lima supports several VM drivers for running guest machines:

The vmType can be specified only on creating the instance.
The vmType of existing instances cannot be changed.

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

The default vmType is QEMU except on macOS 13 or newer,
unless the config is incompatible with VZ. (e.g. cross-architecture emulation)
