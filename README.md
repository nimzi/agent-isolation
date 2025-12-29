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

### (Unsafe) Bake SSH keys into the container image (for Git/GitHub)

Criminally embedding ssh keys from the host so `git` (and GitHub SSH) work from inside the container.

**Security warning:** putting private keys into an image is dangerous:

- Anyone with the image (or access to your Docker registry / cache) can extract your keys.
- Keys may persist in build caches and image layers.
- Rotating keys later requires rebuilding and ensuring old images are deleted everywhere.

Treat this image as **highly sensitive**.

#### What to put in `docker_ssh/` (on the host)

Copy the SSH material you want baked into the image into the `docker_ssh/` folder, for example:

- `id_ed25519` (private key)
- `id_ed25519.pub` (public key)
- `known_hosts` (optional; the entrypoint also adds GitHub host keys)
- `config` (optional)

#### Build/rebuild (bakes keys into the image)

```bash
mkdir -p docker_ssh
cp -a "$HOME/.ssh/id_ed25519" docker_ssh/
cp -a "$HOME/.ssh/id_ed25519.pub" docker_ssh/
cp -a "$HOME/.ssh/known_hosts" docker_ssh/ 2>/dev/null || true
cp -a "$HOME/.ssh/config" docker_ssh/ 2>/dev/null || true
./recreate-container.sh
```

Notes:
- Keys are baked into the image under `/image_ssh/` and copied into `/root/.ssh` at container start (because this repo mounts a persistent volume on `/root`).
- The folder `docker_ssh/` is ignored by git (this repo tracks only a placeholder file to keep the directory present for Docker builds).

## Build and run

**First time setup:**
```bash
./recreate-container.sh
```

This script will:
- Build the Docker image (includes Node.js + tooling for Cursor Agent CLI)
- Create a container (optionally using `.env` if present)
- Mount your project directory to `/work`
- Create a persistent volume for `/root` (home directory)
- Mount your Cursor credentials from `$HOME/.config/cursor` to `/root/.config/cursor`

**Or manually:**
```bash
# Build the image
docker build -t ai-agent-shell .

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

The `container-control.sh` script provides a convenient way to start or stop the Docker container without rebuilding it. This is useful for managing the container lifecycle after initial setup.

**Usage:**
```bash
# Start the container
./container-control.sh start

# Stop the container
./container-control.sh stop
```

**Features:**
- Checks if the container exists before attempting to start/stop
- Verifies container state to avoid unnecessary operations
- Provides clear error messages if the container doesn't exist or operations fail
- Respects the `AI_SHELL_CONTAINER` environment variable for custom container names

**Note:** If the container doesn't exist, you'll need to run `./recreate-container.sh` first to create it.

## Use

```bash
./enter-container.sh
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

**Important:** When rebuilding the image, your volume data persists. Use `./recreate-container.sh` to rebuild while preserving your data.

## Configuration

Environment variables (can be set in `.env` or as container env vars):

- `AI_SHELL_CONTAINER`: Container name (default: `ai-agent-shell`)
- `AI_SHELL_IMAGE`: Image name (default: `ai-agent-shell`)
- `AI_SHELL_VOLUME`: Volume name for `/root` (default: `ai_agent_shell_home`)
- `GH_TOKEN`: Optional token for GitHub CLI (`gh`) authentication
