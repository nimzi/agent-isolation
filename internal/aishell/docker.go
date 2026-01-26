package aishell

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Docker struct {
	Runtime string // "docker" or "podman"
	Timeout time.Duration
	Dir     string // working directory for docker commands (for build context / relative paths)
}

func (d Docker) cmd(ctx context.Context, args ...string) *exec.Cmd {
	runtime := d.Runtime
	if runtime == "" {
		runtime = "docker" // default for backward compatibility
	}
	c := exec.CommandContext(ctx, runtime, args...)
	if d.Dir != "" {
		c.Dir = d.Dir
	}
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c
}

func (d Docker) run(ctx context.Context, args ...string) error {
	return d.cmd(ctx, args...).Run()
}

func (d Docker) runCapture(ctx context.Context, args ...string) (string, error) {
	runtime := d.Runtime
	if runtime == "" {
		runtime = "docker" // default for backward compatibility
	}
	c := exec.CommandContext(ctx, runtime, args...)
	if d.Dir != "" {
		c.Dir = d.Dir
	}
	var out bytes.Buffer
	c.Stdout = &out
	c.Stderr = &out
	err := c.Run()
	if err == nil {
		return out.String(), nil
	}
	msg := strings.TrimSpace(out.String())
	if msg == "" {
		return "", err
	}
	return out.String(), fmt.Errorf("%w: %s", err, msg)
}

func (d Docker) Require() error {
	runtime := d.Runtime
	if runtime == "" {
		runtime = "docker" // default for backward compatibility
	}
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 20*time.Second))
	defer cancel()
	_, err := d.runCapture(ctx, "version")
	if err != nil {
		return fmt.Errorf("%s not available: %w", runtime, err)
	}
	return nil
}

func (d Docker) ContainerExists(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 20*time.Second))
	defer cancel()
	_, err := d.runCapture(ctx, "inspect", name)
	return err == nil
}

func (d Docker) ContainerRunning(name string) bool {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 20*time.Second))
	defer cancel()
	out, err := d.runCapture(ctx, "inspect", "-f", "{{.State.Running}}", name)
	if err != nil {
		return false
	}
	return strings.Contains(strings.ToLower(out), "true")
}

type InspectContainer struct {
	Config struct {
		Image  string            `json:"Image"`
		Labels map[string]string `json:"Labels"`
		Env    []string          `json:"Env"`
	} `json:"Config"`
	State struct {
		Status string `json:"Status"`
	} `json:"State"`
	Mounts []struct {
		Type        string `json:"Type"`
		Source      string `json:"Source"`
		Destination string `json:"Destination"`
	} `json:"Mounts"`
}

// Workdir returns the host path of the /work bind mount, or empty if not found.
func (ic InspectContainer) Workdir() string {
	for _, m := range ic.Mounts {
		if m.Destination == "/work" && m.Type == "bind" {
			return m.Source
		}
	}
	return ""
}

func (d Docker) InspectContainer(name string) (InspectContainer, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 20*time.Second))
	defer cancel()
	out, err := d.runCapture(ctx, "inspect", name)
	if err != nil {
		return InspectContainer{}, fmt.Errorf("docker inspect %q failed: %w", name, err)
	}
	var arr []InspectContainer
	if err := json.Unmarshal([]byte(out), &arr); err != nil {
		return InspectContainer{}, fmt.Errorf("parse docker inspect JSON: %w", err)
	}
	if len(arr) < 1 {
		return InspectContainer{}, errors.New("docker inspect returned empty result")
	}
	return arr[0], nil
}


func (d Docker) Start(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	_, err := d.runCapture(ctx, "start", name)
	return err
}

func (d Docker) Stop(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	_, err := d.runCapture(ctx, "stop", name)
	return err
}

func (d Docker) Remove(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	_, err := d.runCapture(ctx, "rm", name)
	return err
}

func (d Docker) RemoveVolume(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	_, err := d.runCapture(ctx, "volume", "rm", name)
	return err
}

func (d Docker) RemoveImage(name string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	_, err := d.runCapture(ctx, "rmi", name)
	return err
}

func (d Docker) ExecCapture(container string, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	// Use sh for portability across base images (bash may not exist yet).
	return d.runCapture(ctx, "exec", container, "sh", "-c", cmd)
}

func (d Docker) Exec(container string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	return d.run(ctx, append([]string{"exec", container}, args...)...)
}

func (d Docker) ExecTty(container string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	// -t enables nicer output (colors/progress bars) when host has a TTY.
	return d.run(ctx, append([]string{"exec", "-t", container}, args...)...)
}

func (d Docker) InspectMounts(container string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 20*time.Second))
	defer cancel()
	return d.runCapture(ctx, "inspect", "-f", "{{range .Mounts}}{{println .Type .Source \"->\" .Destination}}{{end}}", container)
}

func (d Docker) PSNamesByLabel(key, val string) ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 30*time.Second))
	defer cancel()
	out, err := d.runCapture(ctx, "ps", "-a", "--filter", fmt.Sprintf("label=%s=%s", key, val), "--format", "{{.Names}}")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			names = append(names, ln)
		}
	}
	return names, nil
}

func (d Docker) VolumeNames() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 30*time.Second))
	defer cancel()
	out, err := d.runCapture(ctx, "volume", "ls", "--format", "{{.Name}}")
	if err != nil {
		return nil, err
	}
	var names []string
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			names = append(names, ln)
		}
	}
	return names, nil
}

// NewDocker creates a new Docker instance with the specified runtime
// Validates that runtime is "docker" or "podman"
func NewDocker(runtime string) (Docker, error) {
	if err := validateMode(runtime); err != nil {
		return Docker{}, err
	}
	return Docker{Runtime: runtime}, nil
}

// Compose wraps docker compose / podman-compose commands.
type Compose struct {
	Runtime string        // "docker" or "podman"
	Dir     string        // working directory (where docker-compose.yml lives)
	Timeout time.Duration // default timeout for commands
}

// composeCmd returns the base command and args for compose.
// For docker: "docker compose ..."
// For podman: "podman-compose ..."
func (c Compose) composeCmd() (string, []string) {
	if c.Runtime == ModePodman {
		return "podman-compose", nil
	}
	return "docker", []string{"compose"}
}

// run executes a compose command with stdout/stderr attached.
func (c Compose) run(ctx context.Context, args ...string) error {
	bin, baseArgs := c.composeCmd()
	fullArgs := append(baseArgs, args...)
	cmd := exec.CommandContext(ctx, bin, fullArgs...)
	cmd.Dir = c.Dir
	cmd.Stdin = os.Stdin
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	return cmd.Run()
}

// runCapture executes a compose command and captures output.
func (c Compose) runCapture(ctx context.Context, args ...string) (string, error) {
	bin, baseArgs := c.composeCmd()
	fullArgs := append(baseArgs, args...)
	cmd := exec.CommandContext(ctx, bin, fullArgs...)
	cmd.Dir = c.Dir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out
	err := cmd.Run()
	if err == nil {
		return out.String(), nil
	}
	msg := strings.TrimSpace(out.String())
	if msg == "" {
		return "", err
	}
	return out.String(), fmt.Errorf("%w: %s", err, msg)
}

// Up runs "docker compose up -d" with optional --build flag.
func (c Compose) Up(build bool) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 15*time.Minute))
	defer cancel()
	args := []string{"up", "-d"}
	if build {
		args = append(args, "--build")
	}
	return c.run(ctx, args...)
}

// UpWithBuildArg runs "docker compose up -d --build" with a build arg.
// For docker compose: build args are passed via environment variables (docker compose up doesn't support --build-arg).
// For podman-compose: uses native --build-arg flag which is supported.
func (c Compose) UpWithBuildArg(argName, argValue string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 15*time.Minute))
	defer cancel()

	// Set the build arg as an environment variable for compose to pick up
	// This works for both docker and podman since the compose file references ${BASE_IMAGE}
	os.Setenv(argName, argValue)

	args := []string{"up", "-d", "--build"}
	// podman-compose supports --build-arg directly, so use it for better reliability
	if c.Runtime == ModePodman {
		args = append(args, "--build-arg", argName+"="+argValue)
	}
	return c.run(ctx, args...)
}

// Down runs "docker compose down".
func (c Compose) Down() error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 2*time.Minute))
	defer cancel()
	return c.run(ctx, "down")
}

// DownVolumes runs "docker compose down -v" (removes volumes too).
func (c Compose) DownVolumes() error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 2*time.Minute))
	defer cancel()
	return c.run(ctx, "down", "-v")
}

// Start runs "docker compose start".
func (c Compose) Start() error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 60*time.Second))
	defer cancel()
	return c.run(ctx, "start")
}

// Stop runs "docker compose stop".
func (c Compose) Stop() error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 60*time.Second))
	defer cancel()
	return c.run(ctx, "stop")
}

// Exec runs "docker compose exec <service> <cmd...>".
func (c Compose) Exec(service string, cmd ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 60*time.Second))
	defer cancel()
	args := append([]string{"exec", service}, cmd...)
	return c.run(ctx, args...)
}

// ExecT runs "docker compose exec -T <service> <cmd...>" (no TTY).
func (c Compose) ExecT(service string, cmd ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 60*time.Second))
	defer cancel()
	args := append([]string{"exec", "-T", service}, cmd...)
	return c.run(ctx, args...)
}

// ExecCapture runs "docker compose exec -T <service> <cmd...>" and captures output.
func (c Compose) ExecCapture(service string, cmd ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 60*time.Second))
	defer cancel()
	args := append([]string{"exec", "-T", service}, cmd...)
	return c.runCapture(ctx, args...)
}

// PS runs "docker compose ps -q" and returns container IDs.
func (c Compose) PS() ([]string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(c.Timeout, 30*time.Second))
	defer cancel()
	out, err := c.runCapture(ctx, "ps", "-q")
	if err != nil {
		return nil, err
	}
	var ids []string
	for _, ln := range strings.Split(out, "\n") {
		ln = strings.TrimSpace(ln)
		if ln != "" {
			ids = append(ids, ln)
		}
	}
	return ids, nil
}

// IsRunning checks if the compose service has running containers.
func (c Compose) IsRunning() bool {
	ids, err := c.PS()
	return err == nil && len(ids) > 0
}

// NewCompose creates a Compose instance for the given runtime and directory.
func NewCompose(runtime, dir string) (Compose, error) {
	if err := validateMode(runtime); err != nil {
		return Compose{}, err
	}
	return Compose{Runtime: runtime, Dir: dir}, nil
}

func orDefault[T comparable](v T, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}
