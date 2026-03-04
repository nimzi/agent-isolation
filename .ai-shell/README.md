# ai-shell Configuration

This directory contains the ai-shell configuration for this project.

## Usage

### Initialize and start (recommended)

```bash
ai-shell up
```

### Or use Docker Compose directly

```bash
docker compose up -d --build
```

Or with podman-compose (install via `pipx install podman-compose`):

```bash
podman-compose up -d --build
```

### Enter the container

```bash
ai-shell enter
# or: docker compose exec ai-shell bash
```

### Bootstrap tools (install common utilities)

```bash
docker compose exec ai-shell sh /work/.ai-shell/bootstrap-tools.sh
```

### Setup Git SSH access

```bash
docker compose exec ai-shell sh /work/.ai-shell/setup-git-ssh.sh
```

### Stop the container

```bash
ai-shell stop
# or: docker compose down
```

## Notes

- The parent directory (your project) is mounted at `/work` inside the container
- Scripts are accessed from `/work/.ai-shell/` (this directory, mounted)
- The `/root` directory is persisted in a named volume
- Cursor config is mounted read-only from `~/.config/cursor`
- The `ai-shell` CLI commands (`ai-shell status`, `ai-shell enter`, etc.) work with this container
- Set `AI_SHELL_ENV_FILE` environment variable to inject secrets (e.g., `GH_TOKEN`)

## Customization

You can customize this configuration for your project:

- `Dockerfile`: Change base image, add packages, etc.
- `docker-compose.yml`: Add volume mounts, environment variables, services
- `bootstrap-tools.sh`/`bootstrap-tools.py`: Modify which packages are installed
- `setup-git-ssh.sh`: Customize SSH/git setup

Container name: `ai-agent-shell-21e1dff73f`
