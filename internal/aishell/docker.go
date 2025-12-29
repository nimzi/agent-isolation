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
	Timeout time.Duration
	Dir     string // working directory for docker commands (for build context / relative paths)
}

func (d Docker) cmd(ctx context.Context, args ...string) *exec.Cmd {
	c := exec.CommandContext(ctx, "docker", args...)
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
	c := exec.CommandContext(ctx, "docker", args...)
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
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 20*time.Second))
	defer cancel()
	_, err := d.runCapture(ctx, "version")
	return err
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

func (d Docker) ExecCapture(container string, cmd string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	return d.runCapture(ctx, "exec", container, "bash", "-lc", cmd)
}

func (d Docker) Exec(container string, args ...string) error {
	ctx, cancel := context.WithTimeout(context.Background(), orDefault(d.Timeout, 60*time.Second))
	defer cancel()
	return d.run(ctx, append([]string{"exec", container}, args...)...)
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

func orDefault[T comparable](v T, def T) T {
	var zero T
	if v == zero {
		return def
	}
	return v
}
