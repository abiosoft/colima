---
title: VZ
weight: 2
---

| ⚡ Requirement | macOS >= 13.0 |
| -------------- | ------------- |

"vz" option makes use of native virtualization support provided by macOS Virtualization.Framework.

An example configuration:
{{< tabpane text=true >}}
{{% tab header="CLI" %}}

```bash
colima start --vm-type=vz
```
