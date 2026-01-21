# AI Agent CLI in Docker

[![CI](https://github.com/nimzi/agent-isolation/actions/workflows/ci.yml/badge.svg)](https://github.com/nimzi/agent-isolation/actions/workflows/ci.yml)

**Current version: 0.1.0**

Containerized AI agent CLIs (starting with `cursor-agent`) with a persistent `/root` volume and your project mounted at `/work`.

## What you get

- **Agent CLI(s)**: installed into the persistent `/root` volume so they survive rebuilds
- **Host Cursor auth reuse**: mounts your host Cursor config (`$HOME/.config/cursor`) into the container
- **Dev tools**: `git`, `gh`, `curl`, etc. (installed at runtime via `.ai-shell/bootstrap-tools.sh`)
- **Persistent state**: `/root` is a named Docker volume; `/work` is your bind-mounted project directory
- **Per-project customization**: each project gets its own `.ai-shell/` directory with Dockerfile and scripts

## Setup

**Host OS note:** this setup is currently documented for a **Linux host** (for example, it mounts host Cursor credentials from `~/.config/cursor`). So far, `ai-shell` has only been tested on **Ubuntu 24.04**.

### 1. Prerequisite (for Cursor): Cursor installed + signed in (on the host)

Cursor Agent CLI reads credentials from your host’s Cursor installation. Make sure you’re signed in on the host and that `$HOME/.config/cursor` is populated.

### 1a. Cursor Agent CLI install: what to do if the download method changes

On first container creation, `ai-shell up` tries to install `cursor-agent` inside the container automatically (best-effort). Today it does this by running Cursor’s official installer command inside the container:

```bash
curl https://cursor.com/install -fsSL | bash
```

Because Cursor controls that URL/script, it may change in the future. If the install step fails, `ai-shell up` will warn (but still succeeds). You can use this workflow:

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

On the **first** `ai-shell up` (when the container is created), `ai-shell` runs `.ai-shell/setup-git-ssh.sh` inside the container.

- It generates `~/.ssh/id_ed25519` inside the container (stored in the persistent `/root` volume).
- It adds the public key to your GitHub account via `gh ssh-key add`.
- It configures git to use SSH for GitHub (`url."git@github.com:".insteadOf`).
- If `gh` is not authenticated and no env file was provided, `ai-shell up` will **skip** SSH bootstrap and print next steps (you can authenticate with `gh auth login` inside the container, then re-run `ai-shell up`).

Recommended: create a global env file with `GH_TOKEN` (or authenticate once interactively; the persistent `/root` volume keeps your `gh` login).

## Build and run

### Tested base images

The following `BASE_IMAGE` values (Docker images) have been tested with the runtime bootstrap that installs `python3`, `git`, `gh`, and `ssh` inside the container:

| Base image | Package manager | Result | python3 | git | gh | ssh |
|---|---:|---:|---|---|---|---|
| `ubuntu:24.04` | apt | ✅ | 3.12.3 | 2.43.0 | 2.45.0 | OpenSSH_9.6p1 |
| `debian:12-slim` | apt | ✅ | 3.11.2 | 2.39.5 | 2.23.0 | OpenSSH_9.2p1 |
| `fedora:40` | dnf | ✅ | 3.12.8 | 2.49.0 | 2.65.0 | OpenSSH_9.6p1 |
| `opensuse/leap:15.6` | zypper | ✅ | 3.6.15 | 2.51.0 | 2.78.0 | OpenSSH_9.6p1 |
| `opensuse/tumbleweed` | zypper | ✅ | 3.13.11 | 2.52.0 | 2.83.2 | OpenSSH_10.2p1 |
| `alpine:3.19` | apk | ✅ | 3.11.14 | 2.43.7 | 2.39.2 | OpenSSH_9.6p1 |

Notes:
- **Versions vary by distro** (these are the observed versions from the test run).
- Tumbleweed is rolling-release; versions will change frequently.
- For some distros, `gh` may come from distro repos; if not available, the bootstrap falls back to installing `gh` from an official GitHub CLI release.

### Configure runtime mode (required)

Before first use, run the one-time global setup:

```bash
ai-shell setup
```

This creates global config (`~/.config/ai-shell/config.toml`) and env file (`~/.config/ai-shell/.env`).

Alternatively, configure the container runtime manually:

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

**First time setup (one-time per machine):**
```bash
ai-shell setup
```

This creates:
- Global config (`~/.config/ai-shell/config.toml`) with mode (docker/podman)
- Global env file (`~/.config/ai-shell/.env`) for `GH_TOKEN`

**Initialize a project (per-project):**
```bash
ai-shell init
```

This creates:
- Per-project `.ai-shell/` directory with Dockerfile, scripts, and docker-compose.yml

**Build and start the container:**
```bash
ai-shell up
```


This script will:
- Build the Docker image from `.ai-shell/Dockerfile`
- Create a container (optionally using a global env file if present)
- Mount your project directory to `/work`
- Create a persistent volume for `/root` (home directory)
- Mount your Cursor credentials from `$HOME/.config/cursor` to `/root/.config/cursor` (read-only)
- Bootstrap tools inside the container (installs `python3`, `git`, `gh`, and `ssh`)

### Base image selection (Dockerfile FROM)

`ai-shell` builds its image from `.ai-shell/Dockerfile`. The Dockerfile requires a base image (the `FROM` image) via build-arg `BASE_IMAGE`.

You can provide a base image for `up`/`recreate` either by flag or as an optional positional arg (and it may be an alias):

```bash
# literal base image
ai-shell up --base-image ubuntu:24.04

# alias (user-defined)
ai-shell config alias set ubuntu24 ubuntu:24.04
ai-shell up ubuntu24
```


Note: changing the base image affects builds. Existing containers need `--recreate` to pick up the new image.

### Multiple workdirs (multiple containers)

Each **workdir** gets its own container + `/root` volume. The container/volume names are derived from the canonical (absolute) workdir path.

Examples:

```bash
# Create/start an instance for some other project folder
ai-shell up --workdir /path/to/project

# List all managed instances
ai-shell ls
```

**Or manually:**
```bash
# Build the image (from a project with .ai-shell/ scaffolded)
docker build -t ai-agent-shell --build-arg BASE_IMAGE=python:3.12-slim .ai-shell
```

**Important:** `ai-shell` “metadata” is implemented as **container labels** (e.g. `com.nimzi.ai-shell.managed=true`).
A plain `docker run ... ai-agent-shell` creates a usable container, but it will **not** be detected/managed by `ai-shell`
commands like `ai-shell ls/start/stop/rm` unless you add the expected labels.

If you really want to create the container manually *and* have it be manageable by `ai-shell`, use `ai-shell instance`
to print the correct derived names + labels for your workdir, then pass them to `docker run`:

```bash
# Print the derived container/volume names and labels for this workdir:
ai-shell instance --workdir "$(pwd)"

# Then use the printed values in your docker run. Example shape:
docker run -d \
  --name "<container_from_ai_shell_instance>" \
  --label com.nimzi.ai-shell.managed=true \
  --label com.nimzi.ai-shell.schema=1 \
  --label "com.nimzi.ai-shell.workdir=<canonical_workdir_from_ai_shell_instance>" \
  --label "com.nimzi.ai-shell.instance=<iid_from_ai_shell_instance>" \
  --label "com.nimzi.ai-shell.volume=<volume_from_ai_shell_instance>" \
  -v "$(pwd)":/work \
  -v "<volume_from_ai_shell_instance>":/root \
  -v "$HOME/.config/cursor":/root/.config/cursor:ro \
  --env-file "$HOME/.config/ai-shell/.env" \
  ai-agent-shell
```

**Tip:** When using the `ai-shell` CLI (not manual `docker run`), the container name is usually `ai-agent-shell-<id>` (derived from the workdir). Run `ai-shell ls` to see the `SHORT` id and use `ai-shell enter <short>` / `ai-shell stop <short>` without typing the full container name.

**Note:** `TARGET` prefixes must be unique; if a prefix matches multiple instances, `ai-shell` will error with “ambiguous target” and print candidates.

## Container control script

The `ai-shell` CLI provides a convenient way to start/stop/recreate and enter the container.

**Usage:**
```bash
ai-shell --help
ai-shell status       # affects current directory's instance
ai-shell stop         # affects current directory's instance
ai-shell start        # affects current directory's instance
ai-shell stop --workdir /path/to/project

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

**Note:** If the container doesn't exist, run `ai-shell setup` (once per machine), then `ai-shell init` (per project), then `ai-shell up` to create it.

### Customizing the Container

Each project has its own `.ai-shell/` directory (created by `ai-shell init`) containing:
- `Dockerfile` - modify to change base image, add packages, etc.
- `docker-compose.yml` - add volume mounts, environment variables, services
- `bootstrap-tools.sh` / `bootstrap-tools.py` - modify which packages are installed
- `setup-git-ssh.sh` - customize SSH/git setup

You can commit `.ai-shell/` to version control so team members get the same container setup.

**Using Docker Compose directly:**

```bash
cd .ai-shell
docker compose up -d --build
docker compose exec ai-shell bash
```

Or with podman-compose:

```bash
cd .ai-shell
podman-compose up -d --build
podman-compose exec ai-shell bash
```

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


## Configuration

### `ai-shell setup` (one-time per machine)

`ai-shell setup` creates global configuration:

- `config.toml` (mode + seeded base image aliases)
- a global `.env` file (optionally containing `GH_TOKEN`)

Interactive (TTY):

```bash
ai-shell setup
```

Non-interactive (scripts/CI; defaults to docker):

```bash
ai-shell setup --yes
```

Where it writes (defaults):
- `config.toml`: `$XDG_CONFIG_HOME/ai-shell/config.toml` or `~/.config/ai-shell/config.toml`
- `.env`: `$XDG_CONFIG_HOME/ai-shell/.env` or `~/.config/ai-shell/.env`

Seeded base image aliases:
- `ubu` → `ubuntu:24.04`
- `deb` → `debian:12-slim`
- `fed` → `fedora:40`
- `suse` → `opensuse/leap:15.6`
- `tw` → `opensuse/tumbleweed`
- `alp` → `alpine:3.19`

`GH_TOKEN` behavior:
- Interactive: choose to (1) run a host command (default `gh auth token`), (2) enter a token manually (input hidden), or (3) skip.
- Non-interactive: attempts `gh auth token`; if unavailable/fails, writes a placeholder comment instead.

### `ai-shell init` (per-project)

`ai-shell init` scaffolds per-project configuration:

- `.ai-shell/Dockerfile`
- `.ai-shell/docker-compose.yml`
- `.ai-shell/bootstrap-tools.sh`, `.ai-shell/bootstrap-tools.py`
- `.ai-shell/setup-git-ssh.sh`
- `.ai-shell/README.md`

```bash
ai-shell init
```

Initialize a specific workdir:

```bash
ai-shell init --workdir /path/to/project
```

Where it writes:
- `.ai-shell/`: in the current workdir (or `--workdir`)

Environment variables (can be set in `.env` or as container env vars):

- `AI_SHELL_CONTAINER`: Container base name (default: `ai-agent-shell`)
- `AI_SHELL_IMAGE`: Image name (default: `ai-agent-shell`)
- `AI_SHELL_VOLUME`: Volume base name for `/root` (default: `ai_agent_shell_home`)
- `GH_TOKEN`: Optional token for GitHub CLI (`gh`) authentication

## Roadmap

- **Nushell installation**: automatically install Nushell (`nu`) in the container (likely via the existing runtime bootstrap flow).
- **Nushell OpenAI plugins**: automatically install/configure Nushell plugins like `gpt2099.nu` and `nu.ai`.
- **OpenAI credentials**: provide a first-class way to configure OpenAI credentials (for example `OPENAI_API_KEY`) for tools that need them, ideally integrating with the existing env-file discovery (`$XDG_CONFIG_HOME/ai-shell/.env` or `$HOME/.config/ai-shell/.env`).
- **Mistral via Continue (optional)**: add guidance and/or integration work to connect Mistral through [Continue](https://www.continue.dev); it should be a good option for **OCaml**-focused work.
