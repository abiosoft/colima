---
title: Environment Variables
weight: 80
---

## Environment Variables

This page documents the environment variables used in Colima.

### `COLIMA_HOME`

- **Description**: Specifies the Colima home directory where configurations and data are stored.
- **Default**: `~/.colima`
- **Usage**:
  ```sh
  export COLIMA_HOME=~/.colima-custom
  colima start
  ```

### `COLIMA_PROFILE`

- **Description**: Specifies the name of the Colima profile to use.
- **Default**: `default`
- **Usage**:
  ```sh
  export COLIMA_PROFILE=docker-profile
  colima start
  ```

### `DOCKER_HOST`

- **Description**: Automatically set by Colima to point to the Docker socket. Usually you don't need to set this manually.
- **Default**: Set by Colima when Docker runtime is active
- **Usage**:
  ```sh
  # Automatically set by Colima, but can be overridden
  export DOCKER_HOST=unix://$HOME/.colima/default/docker.sock
  ```

## Lima-specific Variables (Advanced)

Since Colima is built on top of Lima, some advanced Lima variables can still be used for fine-tuning:

### `LIMA_HOME`

- **Description**: Specifies the Lima home directory (used by underlying Lima).
- **Default**: `~/.lima`
- **Usage**:
  ```sh
  export LIMA_HOME=~/.lima-custom
  # Note: Colima manages Lima instances, so this is rarely needed
  ```

### `LIMA_INSTANCE`

- **Description**: Used internally by Colima to manage Lima instances.
- **Default**: Managed by Colima based on profile names
- **Note**: Usually not set manually as Colima handles instance management.

### `LIMA_INSTANCE`

- **Description**: The underlying Lima instance name (managed by Colima).
- **Set by**: Colima based on profile name
- **Example**: `colima-default`, `colima-docker`

## Legacy Lima Variables

For advanced users who need to customize the underlying Lima behavior, some Lima environment variables are still respected. However, these should be used with caution as they may interfere with Colima's management:

- `LIMA_HOME`: Lima home directory (impacts where Colima stores data)
- Various QEMU and virtualization-specific settings
