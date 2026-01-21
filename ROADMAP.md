# Roadmap: Container Runtime SDK Migration

This document outlines potential architectural changes to replace the current CLI-based container orchestration with native Go SDKs.

## Current Architecture

The `ai-shell` CLI currently wraps the `docker`/`podman` CLI via `exec.Command`:

```
ai-shell CLI
    └── internal/aishell/docker.go (thin CLI wrapper)
            ├── exec("docker", args...) 
            └── exec("podman", args...)
```

Post-install scripts run inside containers:
- `bootstrap-tools.sh` / `bootstrap-tools.py` - installs packages (bash, curl, git, gh, ssh, etc.)
- `setup-git-ssh.sh` - generates SSH keys, adds to GitHub, configures git

## Motivation

- **Type safety**: Structured API responses instead of parsing CLI output
- **Better error handling**: SDK errors are typed and detailed
- **Cleaner code**: No string building for CLI arguments

## Constraints

- Must support both Docker and Podman
- Remote daemon support is not required
- Dependency size increase is acceptable

## Options

### Option A: SDK for Orchestration Only (Recommended)

Replace the CLI wrapper with SDK implementations while keeping the existing shell scripts.

**Changes:**
1. Add `github.com/docker/docker` SDK dependency
2. Add `github.com/containers/podman/v5` SDK dependency
3. Define a `ContainerRuntime` interface in Go
4. Implement `DockerRuntime` using Docker SDK
5. Implement `PodmanRuntime` using Podman SDK
6. Replace `docker.go` CLI wrapper with interface-based implementation
7. Scripts remain unchanged, called via SDK's exec API

**Interface sketch:**

```go
type ContainerRuntime interface {
    Ping(ctx context.Context) error
    BuildImage(ctx context.Context, contextDir string, opts BuildOptions) error
    CreateContainer(ctx context.Context, opts CreateOptions) (string, error)
    StartContainer(ctx context.Context, id string) error
    StopContainer(ctx context.Context, id string) error
    RemoveContainer(ctx context.Context, id string) error
    InspectContainer(ctx context.Context, id string) (ContainerInfo, error)
    ExecInContainer(ctx context.Context, id string, cmd []string, opts ExecOptions) (ExecResult, error)
    // ... volumes, images, etc.
}
```

**Pros:**
- Cleaner Go code with proper types
- Structured error handling
- Scripts are battle-tested and readable for their purpose

**Cons:**
- Still shipping script files
- Two SDKs to maintain/update

**Effort:** Medium

---

### Option B: Full Go Implementation

Move all script logic into Go and use SDKs throughout. Eliminate shell scripts entirely.

**Changes:**
1. Everything from Option A, plus:
2. Rewrite `bootstrap-tools.py` logic in Go:
   - Detect package manager (apt, dnf, yum, zypper, pacman, apk)
   - Map package names per distro
   - Run install commands via SDK exec
3. Rewrite `setup-git-ssh.sh` logic in Go:
   - Generate SSH keys via exec(`ssh-keygen`)
   - Add GitHub to known_hosts
   - Call `gh ssh-key add` via exec
   - Configure git via exec(`git config`)
   - Test SSH connection with retries
4. Remove embedded scripts from `internal/aishell/scripts/` (bootstrap-tools.sh, bootstrap-tools.py, setup-git-ssh.sh)
5. Simplify Dockerfile (no script copying)

**Pros:**
- Single language for all logic
- No script files to distribute
- Easier to unit test (mock the runtime interface)
- Full control over error handling and output

**Cons:**
- Significantly more Go code (~400+ lines to replace scripts)
- Shell is arguably more readable for "run these commands" style code
- Package manager detection/mapping is tedious in Go

**Effort:** High

---

### Option C: Hybrid - SDK + Embedded Scripts (IMPLEMENTED)

**Status:** The script embedding portion of this option has been implemented. Scripts are now embedded in the binary via `//go:embed` (see `internal/aishell/scripts/`) and scaffolded to `.ai-shell/` per-project by `ai-shell init`. The SDK migration (replacing CLI wrapper with native Go SDKs) remains as future work.

Use SDKs for orchestration, but embed scripts as Go string constants or `embed.FS`.

**Changes:**
1. Everything from Option A
2. Use `//go:embed` to include script files in the binary
3. Write scripts to container at runtime (or pipe to exec stdin)
4. Remove dependency on finding script files via `resolveHome()`

**Pros:**
- Single binary distribution (no separate script files)
- Scripts remain readable shell code
- SDK benefits for orchestration

**Cons:**
- Slight complexity in script delivery mechanism
- Scripts harder to edit without recompiling

**Effort:** Medium

---

## Recommendation

**Option A** provides the best balance:
- SDK benefits for container orchestration (the complex part)
- Keep scripts for in-container setup (where shell excels)
- Lowest risk and effort

If single-binary distribution is important, consider **Option C** as an enhancement to Option A.

**Option B** is only worthwhile if you have strong reasons to eliminate shell scripts entirely (e.g., Windows container support, strict single-language policy).

---

## Implementation Phases (Option A)

### Phase 1: Interface Definition
- Define `ContainerRuntime` interface
- Define supporting types (`BuildOptions`, `CreateOptions`, `ContainerInfo`, etc.)

### Phase 2: Docker SDK Implementation
- Implement `DockerRuntime`
- Wire up to existing CLI commands
- Test with `ai-shell config set-mode docker`

### Phase 3: Podman SDK Implementation
- Implement `PodmanRuntime`
- Handle API differences (Podman's REST API vs Docker's)
- Test with `ai-shell config set-mode podman`

### Phase 4: Cleanup
- Remove old `docker.go` CLI wrapper
- Update tests
- Update documentation

---

## Dependencies

Docker SDK:
```
go get github.com/docker/docker@latest
```

Podman SDK:
```
go get github.com/containers/podman/v5@latest
```

Note: Both SDKs have significant transitive dependencies. Expect `go.sum` to grow substantially.
