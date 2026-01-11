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

func (d Docker) BuildImage(image string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 10*time.Minute))
	defer cancel()
	return d.run(ctx, "build", "-t", image, ".")
}

func (d Docker) BuildImageWithArgs(image string, extraArgs ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 10*time.Minute))
	defer cancel()
	args := []string{"build", "-t", image}
	args = append(args, extraArgs...)
	args = append(args, ".")
	return d.run(ctx, args...)
}

func (d Docker) RunDetached(args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 2*time.Minute))
	defer cancel()
	// capture output to avoid printing container IDs
	_, err := d.runCapture(ctx, append([]string{"run", "-d"}, args...)...)
	return err
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

func orDefault[T comparable](v T, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}
