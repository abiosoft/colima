# Colima Agent Skill

`SKILL.md` (plus the files in `references/`) is an **Agent Skill** — Markdown with YAML
frontmatter that an AI coding assistant loads on demand to give accurate, version-aware Colima
help (install, runtimes, config, troubleshooting, and bootstrap/CI scripting) instead of generic
Docker advice. It is plain documentation, distilled from this repo's own `docs/`; nothing executes.

## How to use it

The skill is discovered from the assistant's *skills directory*, where each skill lives in its own
folder. Install it as a skill named `colima`:

### Claude Code

```sh
# user-level (available in every project)
cp -R skills ~/.claude/skills/colima

# or project-level
cp -R skills <your-project>/.claude/skills/colima
```

Start a new session; the assistant loads the skill automatically when your task involves Colima
(e.g. *"my colima docker daemon isn't reachable from IntelliJ"*, *"write a CI script that brings
colima up with kubernetes"*).

### Other agents (Cursor, Kimi Code, …)

Any tool that supports the `SKILL.md` format works the same way — copy this folder into that tool's
skills directory, e.g.:

```sh
cp -R skills ~/.kimi-code/skills/colima      # Kimi Code
```

…or point the tool's skills path at it directly (Kimi Code: `kimi --skills-dir <dir-containing colima/>`).

## What's inside

| File | Contents |
|---|---|
| `SKILL.md` | Quick reference + when-to-use (the part the model always sees). |
| `references/install.md` | Homebrew, MacPorts, Nix, Arch, binary, source. |
| `references/configuration.md` | Config files, profiles, `COLIMA_HOME`, env into the VM, Lima overrides. |
| `references/runtimes.md` | Docker, containerd, Kubernetes, Incus, AI models; comparisons. |
| `references/troubleshooting.md` | Daemon socket errors, `Broken` status, mounts, disk, networking, updates. |
| `references/automation.md` | Non-interactive patterns for bootstrap / CI / deploy scripts. |

The content mirrors the canonical docs in [`../docs/`](../docs/); version-gated notes (e.g. `since
vX.Y.Z`) are preserved, so advice stays correct across the versions users run.
