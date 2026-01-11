# AI Agent CLI in Docker

Containerized AI agent CLIs (starting with `cursor-agent`) with a persistent `/root` volume and your project mounted at `/work`.

## What you get

- **Agent CLI(s)**: installed into the persistent `/root` volume so they survive rebuilds
- **Host Cursor auth reuse**: mounts your host Cursor config (`$HOME/.config/cursor`) into the container
- **Dev tools**: `git`, `gh`, `curl`, etc. (installed at runtime via `/docker/bootstrap-tools.sh`)
- **Persistent state**: `/root` is a named Docker volume; `/work` is your bind-mounted project directory

## Setup

**Host OS note:** this setup is currently documented for a **Linux host** (for example, it mounts host Cursor credentials from `~/.config/cursor`). So far, `ai-shell` has only been tested on **Ubuntu 24.04**.

### 1. Prerequisite (for Cursor): Cursor installed + signed in (on the host)

Cursor Agent CLI reads credentials from your host’s Cursor installation. Make sure you’re signed in on the host and that `$HOME/.config/cursor` is populated.

### 1a. Cursor Agent CLI install: what to do if the download method changes

On first container creation, `ai-shell up` tries to install `cursor-agent` inside the container automatically. Today it does this by running Cursor’s official installer command inside the container:

```bash
curl https://cursor.com/install -fsSL | bash
```

Because Cursor controls that URL/script, it may change in the future. If the install step fails (or Cursor moves the installer), use this workflow:

- **Create/start the container without installing the agent**:

```bash
ai-shell up --no-install
```

- **Enter the container**:

```bash
ai-shell enter
```

- **Install `cursor-agent` manually using the current official instructions**:
  - Follow the latest instructions from Cursor (they may provide a different URL, package, or command).
  - After installing, make sure the agent is on `PATH` (many installers place binaries in `~/.local/bin`):

```bash
export PATH="$HOME/.local/bin:$PATH"
command -v cursor-agent
cursor-agent --help
```

- **Persistency note**: the container’s `/root` is a named volume, so once `cursor-agent` is installed inside the container, it should persist across rebuilds/recreates of the container.

### 2. (Optional) GitHub CLI auth via `GH_TOKEN`

If you provide `GH_TOKEN` to the container, `gh` will authenticate non-interactively inside the container.

**Global `.env` (optional, recommended):** `ai-shell up` will look for an env file in:

- `--env-file <path>` (explicit; empty string disables)
- `AI_SHELL_ENV_FILE=<path>`
- `$XDG_CONFIG_HOME/ai-shell/.env`
- `~/.config/ai-shell/.env`

If no env file is found, `ai-shell up` still works; GitHub SSH bootstrap may be deferred until you authenticate `gh` interactively inside the container.

Example `.env`:

```bash
GH_TOKEN=github_pat_your_token_here
```

Quick smoke test (inside the container):

```bash
gh auth status
gh api user --jq .login
```

### Git SSH setup (default; runs on first container creation)

On the **first** `ai-shell up` (when the container is created), `ai-shell` runs `/docker/setup-git-ssh.sh` inside the container.

- It generates `~/.ssh/id_ed25519` inside the container (stored in the persistent `/root` volume).
- It adds the public key to your GitHub account via `gh ssh-key add`.
- It configures git to use SSH for GitHub (`url."git@github.com:".insteadOf`).
- If `gh` is not authenticated and no env file was provided, `ai-shell up` will **skip** SSH bootstrap and print next steps (you can authenticate with `gh auth login` inside the container, then re-run `ai-shell up`).

Recommended: create a global env file with `GH_TOKEN` (or authenticate once interactively; the persistent `/root` volume keeps your `gh` login).

## Build and run

### Tested base images

The following `BASE_IMAGE` values (Docker images) have been tested with the runtime bootstrap that installs `python3`, `git`, `gh`, and `ssh` inside the container:

| Base image | Package manager | Result | python3 | git | gh | ssh | node/npm |
|---|---:|---:|---|---|---|---|---:|
| `ubuntu:24.04` | apt | ✅ | 3.12.3 | 2.43.0 | 2.45.0 | OpenSSH_9.6p1 | ✅ |
| `debian:12-slim` | apt | ✅ | 3.11.2 | 2.39.5 | 2.23.0 | OpenSSH_9.2p1 | ✅ |
| `fedora:40` | dnf | ✅ | 3.12.8 | 2.49.0 | 2.65.0 | OpenSSH_9.6p1 | ✅ |
| `opensuse/leap:15.6` | zypper | ✅ | 3.6.15 | 2.51.0 | 2.78.0 | OpenSSH_9.6p1 | ✅ |
| `alpine:3.19` | apk | ✅ | 3.11.14 | 2.43.7 | 2.39.2 | OpenSSH_9.6p1 | ✅ |

Notes:
- **Versions vary by distro** (these are the observed versions from the test run).
- For some distros, `gh` may come from distro repos; if not available, the bootstrap falls back to installing `gh` from an official GitHub CLI release.

### Configure runtime mode (required)

Before first use, configure the container runtime:

```bash
ai-shell config set-mode <docker|podman>
```

Optional (but recommended): set a default base image and define aliases:

```bash
ai-shell config set-default-base-image python:3.12-slim
ai-shell config alias set ubuntu24 ubuntu:24.04
ai-shell config show
```

### Install

Systemwide:

```bash
make install
```

User install:

```bash
make install PREFIX="$HOME/.local"
```

### Dev

```bash
make build
./bin/ai-shell --help
```

### Run

**First time setup (builds image, creates container, installs cursor-agent):**
```bash
ai-shell up --home "$(pwd)"
```

This script will:
- Build the Docker image
- Create a container (optionally using a global env file if present)
- Mount your project directory to `/work`
- Create a persistent volume for `/root` (home directory)
- Mount your Cursor credentials from `$HOME/.config/cursor` to `/root/.config/cursor` (read-only)
-
- Bootstrap tools inside the container (installs `python3`, `git`, `gh`, and `ssh`, and may install `node/npm` depending on distro packages)

### Base image selection (Dockerfile FROM)

`ai-shell` builds its image from `docker/Dockerfile`. The Dockerfile requires a base image (the `FROM` image) via build-arg `BASE_IMAGE`.

You can provide a base image for `up`/`recreate` either by flag or as an optional positional arg (and it may be an alias):

```bash
# literal base image
ai-shell up --home "$(pwd)" --base-image ubuntu:24.04

# alias (user-defined)
ai-shell config alias set ubuntu24 ubuntu:24.04
ai-shell up --home "$(pwd)" ubuntu24
```

Note: changing the base image affects builds. Existing containers need `--recreate` to pick up the new image.

### Multiple workdirs (multiple containers)

Each **workdir** gets its own container + `/root` volume. The container/volume names are derived from the canonical (absolute) workdir path.

Examples:

```bash
# Create/start an instance for some other project folder
ai-shell up --home "$(pwd)" --workdir /path/to/project

# List all managed instances
ai-shell ls
```

**Or manually:**
```bash
# Build the image
docker build -t ai-agent-shell --build-arg BASE_IMAGE=python:3.12-slim -f docker/Dockerfile docker

# Create and start the container
docker run -d \
    --name ai-agent-shell \
    -v $(pwd):/work \
    -v ai_agent_shell_home:/root \
    -v $HOME/.config/cursor:/root/.config/cursor:ro \
    --env-file ~/.config/ai-shell/.env \
    ai-agent-shell
```

**Tip:** When using the `ai-shell` CLI (not manual `docker run`), the container name is usually `ai-agent-shell-<id>` (derived from the workdir). Run `ai-shell ls` to see the `SHORT` id and use `ai-shell enter <short>` / `ai-shell stop <short>` without typing the full container name.

**Note:** `TARGET` prefixes must be unique; if a prefix matches multiple instances, `ai-shell` will error with “ambiguous target” and print candidates.

## Container control script

The `ai-shell` CLI provides a convenient way to start/stop/recreate and enter the container.

**Usage:**
```bash
ai-shell --help
ai-shell status --home "$(pwd)"       # affects current directory's instance
ai-shell stop --home "$(pwd)"         # affects current directory's instance
ai-shell start --home "$(pwd)"        # affects current directory's instance
ai-shell stop --home "$(pwd)" --workdir /path/to/project

# Or target an instance by SHORT/IID/container prefix:
ai-shell ls
ai-shell enter <short>
ai-shell stop <short>
```

### Destructive cleanup: remove all ai-shell Docker state

To remove **all ai-shell managed containers**, their associated **`/root` volumes**, and any **images those containers use**:

```bash
ai-shell rm --nuke
```

This also attempts to remove orphaned volumes matching the default naming scheme `ai_agent_shell_home_*`.

- By default, `--nuke` prompts for confirmation (`Type NUKE to continue:`).
- If no TTY is available, it refuses unless you pass `--yes`:

```bash
ai-shell rm --nuke --yes
```

**Features:**
- Checks if the container exists before attempting to start/stop
- Verifies container state to avoid unnecessary operations
- Provides clear error messages if the container doesn't exist or operations fail
- Respects the `AI_SHELL_CONTAINER` environment variable for custom container names

**Note:** If the container doesn't exist, run `ai-shell up --home "$(pwd)"` first to create it.

### `--home` vs `--workdir` (important)

- **`--workdir`**: the project directory mounted at `/work` (this defines the instance identity).
- **`--home` / `AI_SHELL_HOME`**: where `ai-shell` finds `docker/Dockerfile` and related scripts (Docker build context).
  - When installed, this is commonly `/usr/local/share/ai-shell` (or `~/.local/share/ai-shell`).
  - Env-file discovery for `GH_TOKEN` is global (see above) and is **not** tied to `--home`.

## Use

```bash
ai-shell ls
ai-shell enter <short>
# then inside the container:
cursor-agent --help
```

## Authentication

### Cursor Agent CLI
- **No API key required** - Uses credentials from host machine
- Credentials mounted from `$HOME/.config/cursor` on host to `/root/.config/cursor` in container
- Requires Cursor to be installed and configured on your host machine
- Credentials are read-only mounted (changes in container don't affect host)

### Git + GitHub CLI (inside container)

- `git` is available in the image.
- `gh` (GitHub CLI) is installed in the image, but it still needs authentication for API operations (typically `gh auth login` inside the container or setting `GH_TOKEN`).

## Container State

The container persists data in two locations:

1. **Project directory** (`/work`): Files created here appear in your local directory
2. **Docker volume** (`ai_agent_shell_home` → `/root`): Home directory, configs, and installed packages

**Important:** When rebuilding the image, your volume data persists. Use `ai-shell up --recreate` to rebuild while preserving your data (add `--home "$(pwd)"` if you're using the repo as the Docker build context).

## Configuration

Environment variables (can be set in `.env` or as container env vars):

- `AI_SHELL_CONTAINER`: Container base name (default: `ai-agent-shell`)
- `AI_SHELL_IMAGE`: Image name (default: `ai-agent-shell`)
- `AI_SHELL_VOLUME`: Volume base name for `/root` (default: `ai_agent_shell_home`)
- `GH_TOKEN`: Optional token for GitHub CLI (`gh`) authentication
