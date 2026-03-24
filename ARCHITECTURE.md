# Architecture / Mental Model (ai-shell)

This document summarizes the **internal invariants** and **runtime contract** of `ai-shell`, so future changes are fast and low-risk. It is intended to complement `README.md` (user workflow) with implementation-level details.

## What `ai-shell` is

`ai-shell` manages **one container + one persistent `/root` volume per workdir**. The project workdir is bind-mounted to `/work`. Host Cursor credentials are mounted **read-only** into `/root/.config/cursor`. Host Claude credentials are mounted **read-only** into `/root/.claude`.

The container image is built from `.ai-shell/Dockerfile`, which is rendered from a **family-specific template** (`apt`, `dnf`, `zypper`, or `apk`) embedded in the `ai-shell` binary. The chosen family comes from the **base image alias** in global config (each alias maps to a Docker image ref plus a family). Core packages are installed at **image build time** via that template. The container runs as a long-lived “toolbox” (`tail -f /dev/null`) that you enter via `ai-shell enter`.

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
- `RandomIID()` generates a random 10-char hex iid via `crypto/rand` — used by `ai-shell regen`.

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

- label `net.datatheory.ai-shell.managed=true` must exist
- label `net.datatheory.ai-shell.workdir` must exactly match the canonical workdir

This is enforced by `requireManaged()` in `internal/aishell/cli.go`.

### Labels (the “schema”)

`ai-shell up` sets these labels on container creation:

- `net.datatheory.ai-shell.managed=true`
- `net.datatheory.ai-shell.schema=1`
- `net.datatheory.ai-shell.workdir=<canonical workdir>`
- `net.datatheory.ai-shell.instance=<iid>`
- `net.datatheory.ai-shell.volume=<volume name>`

See `buildLabels()` in `internal/aishell/cli.go`.

## Configuration: runtime mode + env-file resolution

### Runtime mode is mandatory

`ai-shell` must be configured to use **docker** or **podman**:

```bash
ai-shell config set-mode <docker|podman>
```

Mechanics (`internal/aishell/config.go`, `internal/aishell/env.go`):

- Config file format: TOML (`config.toml`) containing at least:
  - `mode` (`docker` or `podman`)
  - `default-base-image` (an **alias name** that must exist under `base-image-aliases`; used when `init` / `up` / `regen` do not pass another alias)
  - `base-image-aliases` (map of alias → `{ image = "...", family = "apt|dnf|zypper|apk" }`; each alias’s `family` selects which embedded Dockerfile template is used)
- Older `config.toml` files that used the previous alias shape (`alias → bare image string`) are **not** migrated automatically; replace the file with the new format.
- Config file path is independent from env-file discovery:
  - `$XDG_CONFIG_HOME/ai-shell/config.toml` (preferred)
  - `~/.config/ai-shell/config.toml` (fallback)
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
- **Side effects**: writes `config.toml` with `0600` permissions

### `config alias set <alias> <image> <family>`

- **Inputs**: three args: alias key, Docker image reference, and package-manager family (`apt`, `dnf`, `zypper`, or `apk`).
- **Side effects**: updates `base-image-aliases` in `config.toml`.

### `init`

- Scaffolds `.ai-shell/` with the files listed under “Packaging / asset discovery”.
- The base image must be given as an **alias** (`--base-image` or the configured default); bare image refs are rejected (`resolveBaseImage` / `chooseBaseImage` only accept keys present in `base-image-aliases`).
- `docker-compose.override.yml` is written only if it does not already exist (never overwritten even with `--force`).
- `--force` overwrites managed project files, but the **Dockerfile** is updated in a marker-aware way: if both AI-SHELL marker lines are present, only the auto-generated block between them is replaced and anything **below** the closing marker is kept. If markers are missing, `init --force` overwrites the whole Dockerfile and prints a warning.

### `regen`

- Rewrites only `docker-compose.yml` with a new random iid.
- `--base-image <alias>` is **required** (no default); the value must be a defined alias (resolved to the real image for the compose build-arg default).
- Collision-checks the new iid against all existing managed container iids; retries until unique.
- Never touches `docker-compose.override.yml`, `Dockerfile`, scripts, or `README.md`.

### `up` / `recreate`

`recreate` is an alias for `up --recreate`.

Inputs:

- `--workdir` (instance identity; default cwd)
- `--cursor-config` (host cursor dir; default `~/.config/cursor`)
- `--env-file` (optional env-file injection; see “Global env file resolution”)
- `--base-image` or optional positional `ALIAS` (must be a configured alias name, not a bare image ref)
- `--no-build`
- `--no-install` (skip all agent installs)
- `--no-install-cursor` (skip cursor-agent only)
- `--no-install-claude` (skip claude code only)
- `--recreate` (or `recreate` command)

Preconditions:

- Configured mode (via `ensureConfig()` pre-run)
- Runtime available (`docker version` / `podman version`)
- `.ai-shell/` directory must exist in workdir (run `ai-shell setup` once, then `ai-shell init` per project)

Main behavior (`newUpCmd()` in `internal/aishell/cli.go`):

- Verify `.ai-shell/` exists (fail with "Run 'ai-shell init' first" if missing; requires `ai-shell setup` first)
- Use `.ai-shell/` as build context
- Instantiate runtime adapter: `NewDocker(getRuntimeMode())`, set `d.Dir = dockerDir`, require runtime via `d.Require()`
- Resolve instance (canonical workdir, iid, container name, volume name)
- Ensure host cursor dir exists (`ensureCursorConfigDir()` creates it if missing)
- Resolve env-file args and print a warning if none/disabled
- If `--recreate` and container exists: stop+remove the container
- `docker compose up` (with `--build` unless `--no-build`): when building, resolve `BASE_IMAGE` from the chosen **alias** via `chooseBaseImage` / `resolveBaseImage` (bare image refs are rejected) and pass it as a build-arg; the checked-in Dockerfile was generated for that alias’s family at `init` time. The running service gets labels (managed/workdir/iid/volume), bind mounts (`<workdir>:/work`, `<cursorDir>:/root/.config/cursor:ro`, `<claudeDir>:/root/.claude:ro`), the named `<volume>:/root`, optional `--env-file` injection, and the project image.
- After the container is up: run `/work/.ai-shell/bootstrap-tools.sh` (embedded as an **empty** hook; add commands for optional runtime-only steps)
- Install `cursor-agent` unless `--no-install` or `--no-install-cursor`:
  - checks `command -v cursor-agent`
  - if missing: runs `curl https://cursor.com/install -fsSL | bash`
  - appends `~/.local/bin` to `~/.bashrc` (best-effort)
- Install `claude` (Claude Code) unless `--no-install` or `--no-install-claude`:
  - checks `command -v claude`
  - if missing: runs `curl -fsSL https://claude.ai/install.sh | bash`
  - appends `~/.local/bin` to `~/.bashrc` (best-effort)
- Run `/work/.ai-shell/setup-git-ssh.sh`:
  - **If an env file was injected**: “fail fast” (errors if ssh setup fails) with redacted, truncated output
  - **If no env file**:
    - if `gh auth status` fails: print next steps; continue (container is still usable)
    - if already authenticated: attempt SSH bootstrap; error if it fails
- If container already exists:
  - `requireManaged()` guardrail
  - start if needed
  - if SSH setup appears incomplete, retry it when `gh auth status` passes; otherwise warn
- At the end: re-check/install `cursor-agent` and `claude` again unless skipped via flags

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
- `claude` (Claude Code) exists and responds to `--version`
- `/root/.claude` is present/readable
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

### Embedded Dockerfile templates (`export.go`)

`internal/aishell/export.go` embeds four templates under `internal/aishell/scripts/`:

- `Dockerfile.apt.tmpl`, `Dockerfile.dnf.tmpl`, `Dockerfile.zypper.tmpl`, `Dockerfile.apk.tmpl`

`generateDockerfile(baseImage, family string)` picks the template by `family`, renders it with `text/template` (data includes the resolved `FROM` image), and wraps the result with marker lines:

- `# === AI-SHELL AUTO-GENERATED — DO NOT EDIT BELOW THIS LINE ===`
- `# === AI-SHELL AUTO-GENERATED — DO NOT EDIT ABOVE THIS LINE ===`

`exportFiles(outputDir, workdir, cfg, baseImage, family string, force bool)` writes the project `.ai-shell/` tree. For the Dockerfile, `spliceDockerfile(existing, newAutoSection)` replaces only the marked block when `init --force` refreshes an existing file, preserving user additions **after** the closing marker. On first init (no markers yet), the file is written as auto-generated content plus a small default “customize below” stub from `defaultCustomSection()`.

Package-manager–appropriate base packages (e.g. `bash`, `curl`, `git`, `gh`, OpenSSH client, CA certs) are installed in the **auto-generated** layer at **build time**. The alias’s `family` must match the base image’s package manager; otherwise the build is misconfigured.

### `.ai-shell/Dockerfile` (in the project)

The committed Dockerfile is the rendered template section (between markers) plus optional user lines below the closing marker. The effective `FROM` / package installs come from the alias’s `family` at `ai-shell init` time.

Important: **`cursor-agent` and `claude` remain intentionally NOT installed in the Dockerfile** because `/root` is a named volume; they are installed on `ai-shell up` (unless skipped with flags) so they persist under `/root`.

### `.ai-shell/bootstrap-tools.sh`

Shipped as an **empty** customization hook (embedded from `scripts/bootstrap-tools.sh`). It is executed on every successful `ai-shell up` after the container is up; use it only for steps that must run at **container runtime** rather than image build. `bootstrap-tools.py` was removed.

### `/work/.ai-shell/setup-git-ssh.sh`

Bootstrap GitHub SSH auth inside the container:

- Requires `gh` installed and authenticated (`gh auth status`)
- Ensures `~/.ssh` exists with correct perms
- Generates `~/.ssh/id_ed25519` if missing
- Adds `github.com` to `known_hosts`
- Adds the public key to GitHub if not already present
- Configures git rewrite: `url."git@github.com:".insteadOf "https://github.com/"`
- Tests SSH with retries to avoid first-run flakiness from propagation delay

## Packaging / asset discovery

Embedded assets are pulled into the `ai-shell` binary via `//go:embed` in `internal/aishell/export.go`:

- `scripts/Dockerfile.apt.tmpl`, `Dockerfile.dnf.tmpl`, `Dockerfile.zypper.tmpl`, `Dockerfile.apk.tmpl`
- `scripts/bootstrap-tools.sh` (empty hook)
- `scripts/setup-git-ssh.sh`

`ai-shell setup` creates global config (`~/.config/ai-shell/config.toml` and `.env`).

`ai-shell init` scaffolds `.ai-shell/` in the workdir with:
- `Dockerfile` (marker-bounded auto-generated section + optional user tail)
- `docker-compose.yml` (auto-generated; never edit by hand)
- `docker-compose.override.yml` (user-editable; never overwritten by ai-shell)
- `bootstrap-tools.sh`
- `setup-git-ssh.sh`
- `README.md`

`docker-compose.yml` and `docker-compose.override.yml` are automatically merged by `docker compose` / `podman-compose` — no `-f` flags needed.

`ai-shell regen --base-image <alias>` regenerates only `docker-compose.yml` with a new random iid (collision-checked against existing managed containers), leaving all other files intact.

`ai-shell up` requires `.ai-shell/` to exist and uses it as the Docker build context.

`make install` installs:

- binary: `$(PREFIX)/bin/ai-shell` (single binary, no external assets needed)

## Known risk areas / sharp edges

- External installer URLs: `curl https://cursor.com/install | bash` and `curl https://claude.ai/install.sh | bash` may change/break; `README.md` documents manual fallbacks.
- Network restrictions (e.g. port 22 blocked) can cause SSH setup failures; `up` runs bootstrap (empty by default), then tries to install agent CLIs so the container remains useful even when SSH is blocked.
- Dockerfile without the AI-SHELL marker pair: `init --force` cannot splice and replaces the **entire** file (with a warning), discarding any previous Dockerfile content.
- Wrong `family` for an alias (e.g. `apk` family with a Debian image) yields a broken or surprising image build; families must match the base image’s package manager.
- Secret redaction is deliberately simple (`TOKEN=`/`KEY=` line patterns); avoid printing env-file contents in errors elsewhere.

