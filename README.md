# AI Agent CLI in Docker

Containerized AI agent CLIs (starting with `cursor-agent`) with a persistent `/root` volume and your project mounted at `/work`.

## What you get

- **Agent CLI(s)**: installed into the persistent `/root` volume so they survive rebuilds
- **Host Cursor auth reuse**: mounts your host Cursor config (`$HOME/.config/cursor`) into the container
- **Dev tools**: `git`, `gh`, `curl`, `jq`, `ripgrep`, etc.
- **Persistent state**: `/root` is a named Docker volume; `/work` is your bind-mounted project directory

## Setup

### 1. Prerequisite (for Cursor): Cursor installed + signed in (on the host)

Cursor Agent CLI reads credentials from your host’s Cursor installation. Make sure you’re signed in on the host and that `$HOME/.config/cursor` is populated.

### 2. (Optional) GitHub CLI auth via `GH_TOKEN`

If you create a `.env` file containing `GH_TOKEN`, `gh` will authenticate non-interactively inside the container.

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
- It fails `ai-shell up` if GitHub auth is not available (so you don't end up with a half-working git setup).

Recommended: set `GH_TOKEN` in `.env` before running `ai-shell up`.

## Build and run

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
- Build the Docker image (includes Node.js + tooling for Cursor Agent CLI)
- Create a container (optionally using `.env` if present)
- Mount your project directory to `/work`
- Create a persistent volume for `/root` (home directory)
- Mount your Cursor credentials from `$HOME/.config/cursor` to `/root/.config/cursor`

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
docker build -t ai-agent-shell -f docker/Dockerfile docker

# Create and start the container
docker run -d \
    --name ai-agent-shell \
    -v $(pwd):/work \
    -v ai_agent_shell_home:/root \
    -v $HOME/.config/cursor:/root/.config/cursor \
    --env-file .env \
    ai-agent-shell
```

## Container control script

The `ai-shell` CLI provides a convenient way to start/stop/recreate and enter the container.

**Usage:**
```bash
ai-shell --help
ai-shell status --home "$(pwd)"
ai-shell stop --home "$(pwd)"     # affects current directory's instance
ai-shell start --home "$(pwd)"    # affects current directory's instance
ai-shell stop --home "$(pwd)" --workdir /path/to/project
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

## Use

```bash
ai-shell enter --home "$(pwd)"
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

**Important:** When rebuilding the image, your volume data persists. Use `ai-shell up --recreate --home "$(pwd)"` to rebuild while preserving your data.

## Configuration

Environment variables (can be set in `.env` or as container env vars):

- `AI_SHELL_CONTAINER`: Container base name (default: `ai-agent-shell`)
- `AI_SHELL_IMAGE`: Image name (default: `ai-agent-shell`)
- `AI_SHELL_VOLUME`: Volume base name for `/root` (default: `ai_agent_shell_home`)
- `GH_TOKEN`: Optional token for GitHub CLI (`gh`) authentication
