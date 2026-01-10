# Architecture / Mental Model (ai-shell)

This document summarizes the **internal invariants** and **runtime contract** of `ai-shell`, so future changes are fast and low-risk. It is intended to complement `README.md` (user workflow) with implementation-level details.

## What `ai-shell` is

`ai-shell` manages **one container + one persistent `/root` volume per workdir**. The project workdir is bind-mounted to `/work`. Host Cursor credentials are mounted **read-only** into `/root/.config/cursor`.

The container image is built from `docker/Dockerfile`. The container runs as a long-lived “toolbox” (`tail -f /dev/null`) that you enter via `ai-shell enter`.

## Key invariants (identity + safety)

### Workdir is identity

The “instance identity” is the **canonicalized workdir path**:

- `CanonicalWorkdir(p)` (`internal/aishell/workdir.go`):
  - default is current directory if `p == ""`
  - expands `~`
  - `filepath.Abs` then `filepath.EvalSymlinks`
  - must exist and be a directory

### Deterministic instance id

- `InstanceID(workdir)` = `sha256(workdir)` encoded to hex, truncated to **10 chars**
- Used for naming and labeling.

### Naming scheme

Derived from the canonical workdir:

- **container name**: `<containerBase>-<iid>`
- **volume name** (for `/root`): `<volumeBase>_<iid>`

Defaults (overrideable):

- `containerBase`: `AI_SHELL_CONTAINER` or `ai-agent-shell`
- `image`: `AI_SHELL_IMAGE` or `ai-agent-shell`
- `volumeBase`: `AI_SHELL_VOLUME` or `ai_agent_shell_home`

See `internal/aishell/constants.go` and `resolveBases()` in `internal/aishell/cli.go`.

### Managed-container safety checks

Before mutating an existing container derived from a workdir, `ai-shell` verifies it is **managed**:

- label `com.nimzi.ai-shell.managed=true` must exist
- label `com.nimzi.ai-shell.workdir` must exactly match the canonical workdir

This is enforced by `requireManaged()` in `internal/aishell/cli.go`.

### Labels (the “schema”)

`ai-shell up` sets these labels on container creation:

- `com.nimzi.ai-shell.managed=true`
- `com.nimzi.ai-shell.schema=1`
- `com.nimzi.ai-shell.workdir=<canonical workdir>`
- `com.nimzi.ai-shell.instance=<iid>`
- `com.nimzi.ai-shell.volume=<volume name>`

See `buildLabels()` in `internal/aishell/cli.go`.

## Configuration: runtime mode + env-file resolution

### Runtime mode is mandatory

`ai-shell` must be configured to use **docker** or **podman**:

```bash
ai-shell config set-mode <docker|podman>
```

Mechanics (`internal/aishell/config.go`, `internal/aishell/env.go`):

- Config file format: JSON (`config.json`) containing at least:
  - `mode` (`docker` or `podman`)
  - `defaultBaseImage` (Dockerfile `FROM` image; passed as build-arg `BASE_IMAGE`)
  - `baseImageAliases` (map of alias → docker image reference)
- Config file path is independent from env-file discovery:
  - `$XDG_CONFIG_HOME/ai-shell/config.json` (preferred)
  - `~/.config/ai-shell/config.json` (fallback)
- `ensureConfig()`:
  - if config exists: read and validate
  - if missing and **TTY**: prompts for docker vs podman and writes config
  - if missing and **non-TTY**: errors with guidance (won’t prompt)

The root command enforces `ensureConfig()` via `PersistentPreRunE`, except for `ai-shell config ...` commands which intentionally skip the check.

### Global env file resolution

The env file is used to pass variables into the container at creation time (most importantly `GH_TOKEN` for non-interactive `gh` auth).

Resolution order (`resolveEnvFileArgs()` in `internal/aishell/env.go`):

1. `--env-file <path>` if the flag was explicitly set
   - **special case**: `--env-file=""` disables env-file injection
   - if a non-empty path is provided and missing: **hard error**
2. `AI_SHELL_ENV_FILE=<path>`
   - if set but missing: **hard error**
3. `$XDG_CONFIG_HOME/ai-shell/.env` if present
4. `~/.config/ai-shell/.env` if present
5. none

Important: when a path is provided via flag/env and is relative, it is resolved relative to the current working directory.

## How targeting works (TARGET argument)

Some commands accept an optional `TARGET` argument (e.g. `start|stop|status|enter|check|rm`).

Target matching is implemented in `internal/aishell/selector.go` and operates over the list of managed containers (`LabelManaged=true`).

Match order (`matchTarget()`):

1. exact container name
2. exact instance id
3. unique instance id prefix
4. unique container name prefix
5. workdir match **only if** it “looks like a path” (contains `/` or starts with `.` or `~`)

Ambiguity is an error: it prints candidate container(s) with iid + container name + workdir.

`ai-shell ls` also computes a `SHORT` iid prefix length that is unique across all listed instances (bounded 4..10) using `uniquePrefixLen()`.

## CLI surface (what each command does)

Entrypoint: `cmd/ai-shell/main.go` calls `aishell.Main()`, which constructs the cobra root in `internal/aishell/cli.go`.

### `config set-mode <docker|podman>`

- **Inputs**: one arg (`docker` or `podman`)
- **Preconditions**: none (config command bypasses `PersistentPreRunE`)
- **Side effects**: writes `config.json` with `0600` permissions

### `up` / `recreate`

`recreate` is an alias for `up --recreate`.

Inputs:

- `--workdir` (instance identity; default cwd)
- `--home` / `AI_SHELL_HOME` (where to find build context; see “Packaging / assets”)
- `--cursor-config` (host cursor dir; default `~/.config/cursor`)
- `--env-file` (optional env-file injection; see “Global env file resolution”)
- `--base-image` or optional positional `BASE_IMAGE_OR_ALIAS` (Dockerfile `FROM` image; may be an alias)
- `--no-build`
- `--no-install`
- `--recreate` (or `recreate` command)

Preconditions:

- Configured mode (via `ensureConfig()` pre-run)
- Runtime available (`docker version` / `podman version`)
- Build context resolvable (see `resolveHome()` / `resolveDockerDir()`)

Main behavior (`newUpCmd()` in `internal/aishell/cli.go`):

- Resolve build context dir (`resolveHome()` → `resolveDockerDir()`)
- Instantiate runtime adapter: `NewDocker(getRuntimeMode())`, set `d.Dir = dockerDir`, require runtime via `d.Require()`
- Resolve instance (canonical workdir, iid, container name, volume name)
- Ensure host cursor dir exists (`ensureCursorConfigDir()` creates it if missing)
- Resolve env-file args and print a warning if none/disabled
- If `--recreate` and container exists: stop+remove the container
- If not `--no-build`: build image (`docker build -t <image> --build-arg BASE_IMAGE=<resolved> .`)
- If container does **not** exist: run it detached with:
  - labels (managed/workdir/iid/volume)
  - mounts:
    - `<workdir>:/work` (bind)
    - `<volume>:/root` (named volume)
    - `<cursorDir>:/root/.config/cursor:ro` (read-only bind)
  - `--env-file <resolved>` if provided/found
  - image name
- Install `cursor-agent` early unless `--no-install`:
  - checks `command -v cursor-agent`
  - if missing: runs `curl https://cursor.com/install -fsSL | bash`
  - appends `~/.local/bin` to `~/.bashrc` (best-effort)
- Run `/docker/setup-git-ssh.sh`:
  - **If an env file was injected**: “fail fast” (errors if ssh setup fails) with redacted, truncated output
  - **If no env file**:
    - if `gh auth status` fails: print next steps; continue (container is still usable)
    - if already authenticated: attempt SSH bootstrap; error if it fails
- If container already exists:
  - `requireManaged()` guardrail
  - start if needed
  - if SSH setup appears incomplete, retry it when `gh auth status` passes; otherwise warn
- At the end: re-check/install `cursor-agent` again unless `--no-install`

### `start [TARGET]` / `stop [TARGET]`

- **Inputs**: optional `TARGET` (see “How targeting works”)
- **Preconditions**: runtime available; config mode set
- **Side effects**:
  - `start`: starts the container (no-op if already running)
  - `stop`: stops the container (no-op if already stopped)
- If no `TARGET`, operates on the container derived from `--workdir` (or cwd) and enforces `requireManaged()`.

### `status [TARGET]`

- Prints workdir / iid / container / image / volume and best-effort mount list via `docker inspect`.
- If no `TARGET`, status is derived from `--workdir` (or cwd); requires managed container if it exists.

### `enter [TARGET]`

- Ensures the container is running (starts it if needed).
- Runs an interactive `bash -l` inside the container using `docker exec`/`podman exec`.
- Uses `syscall.Exec` (process replacement) on non-Windows for better TTY/signal behavior (`execReplace()` in `internal/aishell/util.go`).

### `check [TARGET]`

Sanity checks (must be running):

- `cursor-agent` exists and responds to `--help`
- `/root/.config/cursor` mount is present/readable
- prints `gh auth status` output (redacting simple `TOKEN=`/`KEY=` patterns)

### `instance`

Purely local derivation (does **not** require docker/podman):

- Prints canonical workdir, iid, container, volume, image, and the expected labels.

### `ls`

- Lists all managed containers (`LabelManaged=true`) with:
  - `WORKDIR`, `SHORT` (unique iid prefix), full `IID`, container name, status

### `rm [TARGET]` / `rm --volume`

Default (no args):

- Removes the container derived from `--workdir` (or cwd) if it exists, guarded by `requireManaged()`.
- With `--volume`: also removes the derived `/root` volume.

With `TARGET`:

- Removes the targeted managed container (and optionally the volume named in its `LabelVolume` label).

### `rm --nuke [--yes]`

Destructive cleanup:

- Deletes **all managed containers**, their labeled volumes, plus **orphan** volumes matching `ai_agent_shell_home_` prefix.
- Also tries to delete images those containers use.
- Requires typing `NUKE` on a TTY unless `--yes` is provided; refuses in non-TTY without `--yes`.

## Runtime adapter (docker vs podman)

`internal/aishell/docker.go` is a thin adapter over the docker/podman CLI:

- All operations are `exec.CommandContext` calls.
- `d.Dir` sets the working directory for commands (important for build context).
- Some operations capture stdout/stderr into a single buffer to:
  - avoid printing container IDs (`run -d`),
  - provide actionable errors that include CLI output.
- Timeouts vary by operation (default-ish):
  - `Require`: 20s (`version`)
  - `BuildImage`: 10m
  - `RunDetached`: 2m
  - start/stop/rm: 60s

## Container image + bootstrap script

### `docker/Dockerfile`

Base: `python:3.12-slim` (Python retained for general scripting convenience).

Installs common tools including:

- `bash`, `ca-certificates`, `curl`, `git`, `jq`, `ripgrep`, `gh`
- networking/debug: `iputils-ping`, `dnsutils`, `netcat-traditional`, `procps`, `gnupg`
- Node.js 20 via NodeSource (needed by `cursor-agent` installer/workflow)

Copies `docker/setup-git-ssh.sh` to `/docker/setup-git-ssh.sh` and makes it executable.

Important: **`cursor-agent` is intentionally NOT installed at image build time** because `/root` is a named volume; installing after container creation ensures it persists in the `/root` volume.

### `/docker/setup-git-ssh.sh`

Bootstrap GitHub SSH auth inside the container:

- Requires `gh` installed and authenticated (`gh auth status`)
- Ensures `~/.ssh` exists with correct perms
- Generates `~/.ssh/id_ed25519` if missing
- Adds `github.com` to `known_hosts`
- Adds the public key to GitHub if not already present
- Configures git rewrite: `url."git@github.com:".insteadOf "https://github.com/"`
- Tests SSH with retries to avoid first-run flakiness from propagation delay

## Packaging / asset discovery

`resolveHome()` in `internal/aishell/cli.go` finds the Docker build context:

Priority:

1. `--home`
2. `AI_SHELL_HOME`
3. current directory if it contains `docker/Dockerfile` (or `Dockerfile`)
4. executable directory if it contains `docker/Dockerfile` (or `Dockerfile`)
5. install share dir(s):
   - `$XDG_DATA_HOME/ai-shell`
   - `~/.local/share/ai-shell`
   - `/usr/local/share/ai-shell`
   - `/usr/share/ai-shell`

`make install` installs:

- binary: `$(PREFIX)/bin/ai-shell`
- assets: `$(PREFIX)/share/ai-shell/docker/*`

## Known risk areas / sharp edges

- External installer URL: `curl https://cursor.com/install | bash` may change/break; `README.md` documents a manual fallback.
- Network restrictions (e.g. port 22 blocked) can cause SSH setup failures; `up` tries to install `cursor-agent` first so the container remains useful.
- Secret redaction is deliberately simple (`TOKEN=`/`KEY=` line patterns); avoid printing env-file contents in errors elsewhere.

