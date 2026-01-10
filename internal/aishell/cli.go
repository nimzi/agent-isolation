package aishell

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/spf13/cobra"
)

type Config struct {
	Workdir       string
	Home          string
	ContainerBase string
	Image         string
	VolumeBase    string
}

func Main() int {
	root := newRootCmd()
	if err := root.Execute(); err != nil {
		// Cobra already prints errors for many cases; keep this concise.
		fmt.Fprintln(os.Stderr, err.Error())
		return 1
	}
	return 0
}

func newRootCmd() *cobra.Command {
	cfg := &Config{}

	root := &cobra.Command{
		Use:   "ai-shell",
		Short: "Manage per-workdir ai-shell Docker/Podman containers",
		Long: strings.TrimSpace(`
Manage per-workdir ai-shell Docker/Podman containers.

Workdir is the identity: one container + one /root volume per workdir.

Containerization mode (docker or podman) must be configured before first use:
  ai-shell config set-mode <docker|podman>

Image builds use a configurable base image (Dockerfile FROM). Configure defaults and aliases:
  ai-shell config set-default-base-image <image|alias>
  ai-shell config alias set <alias> <image>

Defaults can be overridden via env vars:
  AI_SHELL_CONTAINER (base name, default: ai-agent-shell)
  AI_SHELL_IMAGE     (default: ai-agent-shell)
  AI_SHELL_VOLUME    (base name, default: ai_agent_shell_home)
`),
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRunE: func(cmd *cobra.Command, args []string) error {
			// Skip config check for config command itself
			// Check if this is a config subcommand by walking up the command tree
			current := cmd
			for current != nil {
				if current.Use == "config" {
					return nil
				}
				current = current.Parent()
			}
			// Ensure config exists before running any command
			_, err := ensureConfig()
			return err
		},
	}

	root.PersistentFlags().StringVar(&cfg.Workdir, "workdir", "", "Target workdir (default: current directory)")
	root.PersistentFlags().StringVar(&cfg.Home, "home", "", "Repo/build context home used to find docker/Dockerfile (or set AI_SHELL_HOME)")
	root.PersistentFlags().StringVar(&cfg.ContainerBase, "container-base", "", "Container base name (overrides AI_SHELL_CONTAINER)")
	root.PersistentFlags().StringVar(&cfg.Image, "image", "", "Image name (overrides AI_SHELL_IMAGE)")
	root.PersistentFlags().StringVar(&cfg.VolumeBase, "volume-base", "", "Volume base name for /root (overrides AI_SHELL_VOLUME)")

	root.AddCommand(newUpCmd(cfg, false))
	root.AddCommand(newUpCmd(cfg, true)) // recreate alias
	root.AddCommand(newStartCmd(cfg))
	root.AddCommand(newStopCmd(cfg))
	root.AddCommand(newStatusCmd(cfg))
	root.AddCommand(newEnterCmd(cfg))
	root.AddCommand(newCheckCmd(cfg))
	root.AddCommand(newInstanceCmd(cfg))
	root.AddCommand(newLsCmd(cfg))
	root.AddCommand(newRmCmd(cfg))
	root.AddCommand(newConfigCmd())

	return root
}

func resolveBases(cfg *Config) (containerBase, image, volumeBase string) {
	containerBase = firstNonEmpty(cfg.ContainerBase, os.Getenv("AI_SHELL_CONTAINER"), DefaultContainerBase)
	image = firstNonEmpty(cfg.Image, os.Getenv("AI_SHELL_IMAGE"), DefaultImage)
	volumeBase = firstNonEmpty(cfg.VolumeBase, os.Getenv("AI_SHELL_VOLUME"), DefaultVolumeBase)
	return containerBase, image, volumeBase
}

// getRuntimeMode reads the runtime mode from config
// This should only be called after ensureConfig() has been run
func getRuntimeMode() string {
	cfg, err := readConfig()
	if err != nil {
		// This should not happen if ensureConfig() was called, but fallback to docker
		return ModeDocker
	}
	return cfg.Mode
}

func resolveInstance(cfg *Config) (workdir, instanceID, container, image, volume string, err error) {
	containerBase, imageName, volumeBase := resolveBases(cfg)
	wd, err := CanonicalWorkdir(cfg.Workdir)
	if err != nil {
		return "", "", "", "", "", err
	}
	containerName, volumeName := NamesFor(wd, containerBase, volumeBase)
	return wd, InstanceID(wd), containerName, imageName, volumeName, nil
}

func resolveHome(cfg *Config) (string, error) {
	// Priority: flag > env > cwd if (docker/)Dockerfile present > executable dir if (docker/)Dockerfile present > install share dir
	if strings.TrimSpace(cfg.Home) != "" {
		return filepath.Abs(expandUser(cfg.Home))
	}
	if env := strings.TrimSpace(os.Getenv("AI_SHELL_HOME")); env != "" {
		return filepath.Abs(expandUser(env))
	}

	hasDockerfile := func(home string) bool {
		if _, err := os.Stat(filepath.Join(home, "docker", "Dockerfile")); err == nil {
			return true
		}
		if _, err := os.Stat(filepath.Join(home, "Dockerfile")); err == nil {
			return true
		}
		return false
	}

	if wd, err := os.Getwd(); err == nil {
		if abs, err := filepath.Abs(wd); err == nil && hasDockerfile(abs) {
			return abs, nil
		}
	}
	if exe, err := os.Executable(); err == nil {
		if dir, err := filepath.Abs(filepath.Dir(exe)); err == nil && hasDockerfile(dir) {
			return dir, nil
		}
	}

	// Default locations for installed Docker build assets.
	// These are populated by `make install` into $(PREFIX)/share/ai-shell.
	if xdg := strings.TrimSpace(os.Getenv("XDG_DATA_HOME")); xdg != "" {
		if home := filepath.Join(expandUser(xdg), "ai-shell"); hasDockerfile(home) {
			return home, nil
		}
	}
	if homeEnv := strings.TrimSpace(os.Getenv("HOME")); homeEnv != "" {
		if home := filepath.Join(expandUser(homeEnv), ".local", "share", "ai-shell"); hasDockerfile(home) {
			return home, nil
		}
	}
	for _, home := range []string{
		"/usr/local/share/ai-shell",
		"/usr/share/ai-shell",
	} {
		if hasDockerfile(home) {
			return home, nil
		}
	}

	return "", errors.New("cannot locate Docker build context; set --home / AI_SHELL_HOME or install assets to /usr/local/share/ai-shell (or ~/.local/share/ai-shell)")
}

func resolveDockerDir(home string) (string, error) {
	d := filepath.Join(home, "docker")
	if _, err := os.Stat(filepath.Join(d, "Dockerfile")); err == nil {
		return d, nil
	}
	// also accept home itself if Dockerfile is there (older layout)
	if _, err := os.Stat(filepath.Join(home, "Dockerfile")); err == nil {
		return home, nil
	}
	return "", fmt.Errorf("cannot locate Dockerfile under %s (expected %s)", home, filepath.Join(home, "docker", "Dockerfile"))
}

func ensureCursorConfigDir(p string) (string, error) {
	p = expandUser(p)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	if err := os.MkdirAll(abs, 0o755); err != nil {
		return "", err
	}
	return abs, nil
}

func requireManaged(d Docker, container string, expectedWorkdir string) error {
	info, err := d.InspectContainer(container)
	if err != nil {
		return err
	}
	labels := info.Config.Labels
	if labels == nil || labels[LabelManaged] != "true" {
		return fmt.Errorf("refusing: container %q is not managed by ai-shell (missing label %s=true)", container, LabelManaged)
	}
	if got := labels[LabelWorkdir]; got != expectedWorkdir {
		return fmt.Errorf("refusing: container %q workdir label mismatch\nexpected: %s\nfound:    %s", container, expectedWorkdir, got)
	}
	return nil
}

func buildLabels(workdir, instanceID, volumeName string) []string {
	return []string{
		"--label", LabelManaged + "=true",
		"--label", LabelSchema + "=1",
		"--label", LabelWorkdir + "=" + workdir,
		"--label", LabelInstance + "=" + instanceID,
		"--label", LabelVolume + "=" + volumeName,
	}
}

func installCursorAgentIfMissing(d Docker, container string) error {
	// Avoid printing installer output; only return errors.
	_, err := d.ExecCapture(container, "command -v cursor-agent >/dev/null 2>&1")
	if err == nil {
		return nil
	}
	// installer can be chatty; best-effort to keep host output minimal
	_, err = d.ExecCapture(container, "curl https://cursor.com/install -fsSL | bash")
	if err != nil {
		return fmt.Errorf("install cursor-agent: %w", err)
	}
	_, _ = d.ExecCapture(container, `echo "export PATH=\"$HOME/.local/bin:$PATH\"" >> ~/.bashrc`)
	return nil
}

func newUpCmd(cfg *Config, aliasRecreate bool) *cobra.Command {
	var cursorConfig string
	var envFile string
	var noBuild bool
	var noInstall bool
	var baseImage string
	var recreate bool

	use := "up"
	short := "Create/start the workdir container (optionally build/install)"
	if aliasRecreate {
		use = "recreate"
		short = "Alias for: up --recreate"
		recreate = true
	}

	cmd := &cobra.Command{
		Use:   use + " [BASE_IMAGE_OR_ALIAS]",
		Short: short,
		Args:  cobra.MaximumNArgs(1),
		PreRunE: func(cmd *cobra.Command, args []string) error {
			home, err := resolveHome(cfg)
			if err != nil {
				return err
			}
			_, err = resolveDockerDir(home)
			return err
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			home, err := resolveHome(cfg)
			if err != nil {
				return err
			}
			dockerDir, err := resolveDockerDir(home)
			if err != nil {
				return err
			}
			runtime, err := NewDocker(getRuntimeMode())
			if err != nil {
				return err
			}
			runtime.Dir = dockerDir
			d := runtime
			if err := d.Require(); err != nil {
				return err
			}

			workdir, iid, container, image, volume, err := resolveInstance(cfg)
			if err != nil {
				return err
			}

			cursorDir, err := ensureCursorConfigDir(cursorConfig)
			if err != nil {
				return err
			}

			envRes, err := resolveEnvFileArgs(envFile, cmd.Flags().Changed("env-file"))
			if err != nil {
				return err
			}
			if envRes.Source == "none" || envRes.Source == "disabled" {
				if envRes.Source == "disabled" {
					fmt.Fprintln(os.Stderr, `Warning: env-file injection disabled via --env-file="".`)
					fmt.Fprintln(os.Stderr)
				}
				fmt.Fprintln(os.Stderr, formatEnvMissingWarning(defaultGlobalEnvFilePath()))
				fmt.Fprintln(os.Stderr)
			}

			if recreate && d.ContainerExists(container) {
				_ = d.Stop(container)
				_ = d.Remove(container)
			}

			if !noBuild {
				appCfg, err := readConfig()
				if err != nil {
					return err
				}
				base, _, _, err := chooseBaseImage(baseImage, args, appCfg)
				if err != nil {
					return err
				}
				if err := d.BuildImageWithArgs(image, "--build-arg", "BASE_IMAGE="+base); err != nil {
					return err
				}
			}

			if !d.ContainerExists(container) {
				args := []string{
					"--name", container,
				}
				args = append(args, buildLabels(workdir, iid, volume)...)
				args = append(args,
					"-v", workdir+":/work",
					"-v", volume+":/root",
					"-v", cursorDir+":/root/.config/cursor:ro",
				)
				args = append(args, envRes.Args...)
				args = append(args, image)

				if err := d.RunDetached(args...); err != nil {
					return err
				}

				// Install cursor-agent as early as possible so the container is still usable
				// even if SSH setup fails (e.g. port 22 blocked on this network).
				if !noInstall {
					if err := installCursorAgentIfMissing(d, container); err != nil {
						return err
					}
				}

				// SSH setup (writes keys into /root persistent volume).
				// If an env file was provided/found, keep "fail fast" behavior.
				if len(envRes.Args) > 0 {
					out, err := d.ExecCapture(container, "/docker/setup-git-ssh.sh")
					if err != nil {
						out = redactSecrets(out)
						out = strings.TrimSpace(out)
						const maxOut = 4000
						if len(out) > maxOut {
							out = out[:maxOut] + "\n...(truncated)"
						}
						msg := strings.TrimSpace(fmt.Sprintf(`
SSH setup failed inside the container.

ai-shell requires SSH for git operations and runs /docker/setup-git-ssh.sh to bootstrap GitHub SSH access.

This script requires GitHub CLI authentication.

How to fix:
  - Create a global env file with GH_TOKEN:
      $XDG_CONFIG_HOME/ai-shell/.env   (preferred)
      ~/.config/ai-shell/.env          (fallback)
    then re-run: ai-shell up --recreate
  - Or authenticate interactively:
      ai-shell enter
      gh auth login
    then re-run: ai-shell up --recreate

If you want to clean up the failed instance:
  - ai-shell rm --volume   (remove container + /root volume for this workdir)
  - ai-shell rm --nuke     (remove ALL ai-shell containers/volumes/images)

Output from /docker/setup-git-ssh.sh:
%s
`, out))
						return errors.New(msg)
					}
				} else {
					// No env file: only attempt SSH setup if gh is already authenticated in the persistent /root volume.
					if _, err := d.ExecCapture(container, "gh auth status >/dev/null 2>&1"); err != nil {
						fmt.Fprintln(os.Stderr, strings.TrimSpace(`
Warning: GitHub CLI (gh) is not authenticated inside the container, so SSH setup was skipped.

Next steps:
  - ai-shell enter            (or: ai-shell enter <short>)
  - inside the container: gh auth login
  - optionally verify: gh auth status

Then finish SSH setup:
  - re-run: ai-shell up            (it will attempt SSH setup if gh auth status passes)
  - or inside the container: /docker/setup-git-ssh.sh
`))
						fmt.Fprintln(os.Stderr)
					} else {
						// Auth already exists (likely from a prior interactive login); proceed with SSH bootstrap.
						if out, err := d.ExecCapture(container, "/docker/setup-git-ssh.sh"); err != nil {
							out = redactSecrets(out)
							out = strings.TrimSpace(out)
							const maxOut = 4000
							if len(out) > maxOut {
								out = out[:maxOut] + "\n...(truncated)"
							}
							msg := strings.TrimSpace(fmt.Sprintf(`
SSH setup failed inside the container (unexpected).

gh appears to be authenticated, but /docker/setup-git-ssh.sh still failed.

Output from /docker/setup-git-ssh.sh:
%s
`, out))
							return errors.New(msg)
						}
					}
				}
			} else {
				if err := requireManaged(d, container, workdir); err != nil {
					return err
				}
				if !d.ContainerRunning(container) {
					if err := d.Start(container); err != nil {
						return err
					}
				}

				// If SSH setup was previously skipped, try again once gh auth exists.
				needsSSH, err := d.ExecCapture(container, `test -f "$HOME/.ssh/id_ed25519" && git config --global --get url."git@github.com:".insteadOf "https://github.com/" >/dev/null 2>&1 && echo OK`)
				if err != nil || !strings.Contains(needsSSH, "OK") {
					if _, err := d.ExecCapture(container, "gh auth status >/dev/null 2>&1"); err != nil {
						fmt.Fprintln(os.Stderr, strings.TrimSpace(`
Warning: GitHub CLI (gh) is not authenticated inside the container, so GitHub SSH setup has not been completed.

Next steps:
  - ai-shell enter
  - gh auth login
  - gh auth status

Then run:
  - /docker/setup-git-ssh.sh
  - or just re-run: ai-shell up
`))
						fmt.Fprintln(os.Stderr)
					} else {
						if out, err := d.ExecCapture(container, "/docker/setup-git-ssh.sh"); err != nil {
							out = redactSecrets(out)
							out = strings.TrimSpace(out)
							const maxOut = 4000
							if len(out) > maxOut {
								out = out[:maxOut] + "\n...(truncated)"
							}
							msg := strings.TrimSpace(fmt.Sprintf(`
SSH setup failed inside the container.

Output from /docker/setup-git-ssh.sh:
%s
`, out))
							return errors.New(msg)
						}
					}
				}
			}

			if !noInstall {
				if err := installCursorAgentIfMissing(d, container); err != nil {
					return err
				}
			}

			fmt.Printf("OK: up: %s\n", container)
			fmt.Printf("workdir: %s\n", workdir)
			return nil
		},
	}

	cmd.Flags().StringVar(&cursorConfig, "cursor-config", "~/.config/cursor", "Host Cursor config directory")
	cmd.Flags().StringVar(&envFile, "env-file", "", "Env file to pass to docker run. Resolution: --env-file, AI_SHELL_ENV_FILE, then $XDG_CONFIG_HOME/ai-shell/.env or ~/.config/ai-shell/.env if present. Optional. Set to empty to disable.")
	cmd.Flags().StringVar(&baseImage, "base-image", "", "Base image for Dockerfile FROM (may be an alias defined via `ai-shell config alias`). Can also be provided as an optional positional argument.")
	cmd.Flags().BoolVar(&noBuild, "no-build", false, "Skip docker build")
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Skip installing cursor-agent")
	if !aliasRecreate {
		cmd.Flags().BoolVar(&recreate, "recreate", false, "Stop/remove and recreate the container if it already exists")
	}

	return cmd
}

func newStartCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "start [TARGET]",
		Short: "Start the container for this workdir",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := NewDocker(getRuntimeMode())
			if err != nil {
				return err
			}
			if err := d.Require(); err != nil {
				return err
			}
			var workdir, container string
			if len(args) == 1 {
				inst, err := resolveTarget(d, args[0])
				if err != nil {
					return err
				}
				workdir = inst.Workdir
				container = inst.Container
			} else {
				var err error
				workdir, _, container, _, _, err = resolveInstance(cfg)
				if err != nil {
					return err
				}
				if !d.ContainerExists(container) {
					return fmt.Errorf("container not found for workdir: %s (run: ai-shell up)", workdir)
				}
				if err := requireManaged(d, container, workdir); err != nil {
					return err
				}
			}
			if d.ContainerRunning(container) {
				fmt.Printf("OK: %q already running.\n", container)
				return nil
			}
			if err := d.Start(container); err != nil {
				return err
			}
			fmt.Printf("OK: started %q.\n", container)
			return nil
		},
	}
}

func newStopCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "stop [TARGET]",
		Short: "Stop the container for this workdir",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := NewDocker(getRuntimeMode())
			if err != nil {
				return err
			}
			if err := d.Require(); err != nil {
				return err
			}
			var workdir, container string
			if len(args) == 1 {
				inst, err := resolveTarget(d, args[0])
				if err != nil {
					return err
				}
				workdir = inst.Workdir
				container = inst.Container
			} else {
				var err error
				workdir, _, container, _, _, err = resolveInstance(cfg)
				if err != nil {
					return err
				}
				if !d.ContainerExists(container) {
					return fmt.Errorf("container not found for workdir: %s", workdir)
				}
				if err := requireManaged(d, container, workdir); err != nil {
					return err
				}
			}
			if !d.ContainerRunning(container) {
				fmt.Printf("OK: %q already stopped.\n", container)
				return nil
			}
			if err := d.Stop(container); err != nil {
				return err
			}
			fmt.Printf("OK: stopped %q.\n", container)
			return nil
		},
	}
}

func newStatusCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "status [TARGET]",
		Short: "Show status for this workdir instance",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := NewDocker(getRuntimeMode())
			if err != nil {
				return err
			}
			if err := d.Require(); err != nil {
				return err
			}
			containerForMounts := ""
			if len(args) == 1 {
				inst, err := resolveTarget(d, args[0])
				if err != nil {
					return err
				}
				running := d.ContainerRunning(inst.Container)
				fmt.Printf("workdir:   %s\n", inst.Workdir)
				fmt.Printf("instance:  %s\n", inst.InstanceID)
				fmt.Printf("container: %s (%s)\n", inst.Container, ternary(running, "running", inst.Status))
				if strings.TrimSpace(inst.Image) != "" {
					fmt.Printf("image:     %s\n", inst.Image)
				}
				if strings.TrimSpace(inst.Volume) != "" {
					fmt.Printf("volume:    %s -> /root\n", inst.Volume)
				}
				containerForMounts = inst.Container
			} else {
				workdir, iid, container, image, volume, err := resolveInstance(cfg)
				if err != nil {
					return err
				}
				exists := d.ContainerExists(container)
				running := exists && d.ContainerRunning(container)
				fmt.Printf("workdir:   %s\n", workdir)
				fmt.Printf("instance:  %s\n", iid)
				fmt.Printf("container: %s (%s)\n", container, ternary(running, "running", ternary(exists, "stopped", "missing")))
				fmt.Printf("image:     %s\n", image)
				fmt.Printf("volume:    %s -> /root\n", volume)
				if !exists {
					return nil
				}
				if err := requireManaged(d, container, workdir); err != nil {
					return err
				}
				containerForMounts = container
			}

			// Best-effort: print mounts if container exists.
			if containerForMounts == "" || !d.ContainerExists(containerForMounts) {
				return nil
			}
			mounts, _ := d.InspectMounts(containerForMounts)
			mounts = strings.TrimSpace(mounts)
			if mounts != "" {
				fmt.Println("mounts:")
				for _, ln := range strings.Split(mounts, "\n") {
					fmt.Printf("  %s\n", ln)
				}
			}
			return nil
		},
	}
}

func newEnterCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "enter [TARGET]",
		Short: "Enter an interactive shell inside the workdir container",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := NewDocker(getRuntimeMode())
			if err != nil {
				return err
			}
			if err := d.Require(); err != nil {
				return err
			}
			var workdir, container string
			if len(args) == 1 {
				inst, err := resolveTarget(d, args[0])
				if err != nil {
					return err
				}
				workdir = inst.Workdir
				container = inst.Container
			} else {
				var err error
				workdir, _, container, _, _, err = resolveInstance(cfg)
				if err != nil {
					return err
				}
				if !d.ContainerExists(container) {
					return fmt.Errorf("container not found for workdir: %s (run: ai-shell up)", workdir)
				}
				if err := requireManaged(d, container, workdir); err != nil {
					return err
				}
			}
			if !d.ContainerRunning(container) {
				if err := d.Start(container); err != nil {
					return err
				}
			}
			_, _ = d.ExecCapture(container, `grep -q "\.local/bin" ~/.bashrc 2>/dev/null || echo "export PATH=\"$HOME/.local/bin:$PATH\"" >> ~/.bashrc`)

			tty := isTTY()
			runtime := getRuntimeMode()
			argsDocker := []string{"exec"}
			if tty {
				argsDocker = append(argsDocker, "-it")
			} else {
				fmt.Fprintln(os.Stderr, "Warning: no TTY available; running non-interactive shell.")
			}
			argsDocker = append(argsDocker, container, "bash", "-l")

			// Replace process for better UX (signals/TTY)
			return execReplace(runtime, argsDocker)
		},
	}
}

func newCheckCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "check [TARGET]",
		Short: "Sanity-check cursor-agent + mounts (and optional gh auth)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := NewDocker(getRuntimeMode())
			if err != nil {
				return err
			}
			if err := d.Require(); err != nil {
				return err
			}
			var workdir, container string
			startHint := "ai-shell start"
			if len(args) == 1 {
				inst, err := resolveTarget(d, args[0])
				if err != nil {
					return err
				}
				workdir = inst.Workdir
				container = inst.Container
				startHint = fmt.Sprintf("ai-shell start %s", args[0])
			} else {
				var err error
				workdir, _, container, _, _, err = resolveInstance(cfg)
				if err != nil {
					return err
				}
				if !d.ContainerExists(container) {
					return fmt.Errorf("container not found for workdir: %s (run: ai-shell up)", workdir)
				}
				if err := requireManaged(d, container, workdir); err != nil {
					return err
				}
			}
			if !d.ContainerRunning(container) {
				return fmt.Errorf("container is not running: %s (run: %s)", container, startHint)
			}

			if _, err := d.ExecCapture(container, "command -v cursor-agent && cursor-agent --help | head -30"); err != nil {
				return fmt.Errorf("cursor-agent not found (run: ai-shell up)")
			}
			fmt.Println("OK: cursor-agent is installed.")

			if _, err := d.ExecCapture(container, "ls -la /root/.config/cursor/ 2>/dev/null | head -50"); err != nil {
				return errors.New("ERROR: /root/.config/cursor is missing; ensure host Cursor is installed/signed in")
			}
			fmt.Println("OK: /root/.config/cursor is mounted.")

			out, _ := d.ExecCapture(container, "gh auth status 2>&1 | head -50")
			out = redactSecrets(out)
			out = strings.TrimSpace(out)
			if out == "" {
				fmt.Println("gh auth (optional): (no output)")
				return nil
			}
			fmt.Println("gh auth (optional):")
			for _, ln := range strings.Split(out, "\n") {
				fmt.Printf("  %s\n", ln)
			}
			return nil
		},
	}
}

func newLsCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "ls",
		Short: "List all ai-shell managed containers",
		RunE: func(cmd *cobra.Command, args []string) error {
			_ = cfg // list ignores current workdir instance
			d, err := NewDocker(getRuntimeMode())
			if err != nil {
				return err
			}
			if err := d.Require(); err != nil {
				return err
			}
			instances, err := listManagedInstances(d)
			if err != nil {
				return err
			}
			if len(instances) == 0 {
				fmt.Println("No ai-shell managed containers found.")
				return nil
			}

			var ids []string
			for _, inst := range instances {
				if inst.InstanceID != "" {
					ids = append(ids, inst.InstanceID)
				}
			}
			shortLen := uniquePrefixLen(ids, 4, 10)

			type row struct {
				workdir   string
				short     string
				iid       string
				container string
				status    string
			}
			var rows []row
			for _, inst := range instances {
				short := "??????????"
				if shortLen > 0 && len(inst.InstanceID) >= shortLen {
					short = inst.InstanceID[:shortLen]
				}
				rows = append(rows, row{
					workdir:   inst.Workdir,
					short:     short,
					iid:       inst.InstanceID,
					container: inst.Container,
					status:    inst.Status,
				})
			}

			wdW, sW, iidW, cW := 6, 5, 3, 9
			for _, r := range rows {
				wdW = max(wdW, len(r.workdir))
				sW = max(sW, len(r.short))
				iidW = max(iidW, len(r.iid))
				cW = max(cW, len(r.container))
			}
			fmt.Printf("%-*s  %-*s  %-*s  %-*s  STATUS\n", wdW, "WORKDIR", sW, "SHORT", iidW, "IID", cW, "CONTAINER")
			for _, r := range rows {
				fmt.Printf("%-*s  %-*s  %-*s  %-*s  %s\n", wdW, r.workdir, sW, r.short, iidW, r.iid, cW, r.container, r.status)
			}
			return nil
		},
	}
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}

func newRmCmd(cfg *Config) *cobra.Command {
	var removeVolume bool
	var nuke bool
	var yes bool
	cmd := &cobra.Command{
		Use:   "rm [TARGET]",
		Short: "Remove the workdir container (or --nuke all ai-shell Docker state)",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			d, err := NewDocker(getRuntimeMode())
			if err != nil {
				return err
			}
			if err := d.Require(); err != nil {
				return err
			}

			if nuke {
				if removeVolume {
					return errors.New("--nuke cannot be combined with --volume")
				}
				if len(args) > 0 {
					return errors.New("--nuke cannot be combined with TARGET")
				}
				if f := cmd.Flag("workdir"); f != nil && f.Changed {
					return errors.New("--nuke cannot be combined with --workdir")
				}

				containers, err := d.PSNamesByLabel(LabelManaged, "true")
				if err != nil {
					return err
				}
				sort.Strings(containers)

				volSet := map[string]struct{}{}
				imgSet := map[string]struct{}{}

				for _, name := range containers {
					info, err := d.InspectContainer(name)
					if err != nil {
						continue
					}
					if info.Config.Labels != nil {
						if v := strings.TrimSpace(info.Config.Labels[LabelVolume]); v != "" {
							volSet[v] = struct{}{}
						}
					}
					if img := strings.TrimSpace(info.Config.Image); img != "" {
						imgSet[img] = struct{}{}
					}
				}

				volumeNames, err := d.VolumeNames()
				if err != nil {
					return err
				}
				orphanPrefix := DefaultVolumeBase + "_"
				for _, v := range volumeNames {
					if strings.HasPrefix(v, orphanPrefix) {
						volSet[v] = struct{}{}
					}
				}

				var volumes []string
				for v := range volSet {
					volumes = append(volumes, v)
				}
				sort.Strings(volumes)

				var images []string
				for img := range imgSet {
					images = append(images, img)
				}
				sort.Strings(images)

				fmt.Println("This will delete the following Docker resources:")
				fmt.Printf("Containers (%d):\n", len(containers))
				if len(containers) == 0 {
					fmt.Println("  (none)")
				} else {
					for _, c := range containers {
						fmt.Printf("  - %s\n", c)
					}
				}
				fmt.Printf("Volumes (%d):\n", len(volumes))
				if len(volumes) == 0 {
					fmt.Println("  (none)")
				} else {
					for _, v := range volumes {
						fmt.Printf("  - %s\n", v)
					}
				}
				fmt.Printf("Images (%d):\n", len(images))
				if len(images) == 0 {
					fmt.Println("  (none)")
				} else {
					for _, img := range images {
						fmt.Printf("  - %s\n", img)
					}
				}

				if len(containers) == 0 && len(volumes) == 0 && len(images) == 0 {
					fmt.Println("Nothing to delete.")
					return nil
				}

				if !yes {
					if !isTTY() {
						return errors.New("refusing to --nuke without a TTY; re-run with --yes")
					}
					fmt.Print("Type NUKE to continue: ")
					line, _ := bufio.NewReader(os.Stdin).ReadString('\n')
					if strings.TrimSpace(line) != "NUKE" {
						fmt.Println("Aborted.")
						return nil
					}
				}

				var warnContainers []string
				var warnVolumes []string
				var warnImages []string
				removedContainers := 0
				removedVolumes := 0
				removedImages := 0

				for _, c := range containers {
					_ = d.Stop(c)
				}
				for _, c := range containers {
					if err := d.Remove(c); err != nil {
						warnContainers = append(warnContainers, fmt.Sprintf("%s: %v", c, err))
						continue
					}
					removedContainers++
				}
				for _, v := range volumes {
					if err := d.RemoveVolume(v); err != nil {
						warnVolumes = append(warnVolumes, fmt.Sprintf("%s: %v", v, err))
						continue
					}
					removedVolumes++
				}
				for _, img := range images {
					if err := d.RemoveImage(img); err != nil {
						warnImages = append(warnImages, fmt.Sprintf("%s: %v", img, err))
						continue
					}
					removedImages++
				}

				fmt.Println("OK: nuke complete.")
				fmt.Printf("Removed: %d/%d containers, %d/%d volumes, %d/%d images\n",
					removedContainers, len(containers),
					removedVolumes, len(volumes),
					removedImages, len(images),
				)
				if len(warnContainers)+len(warnVolumes)+len(warnImages) > 0 {
					fmt.Println("Warnings (some resources could not be removed):")
					for _, w := range warnContainers {
						fmt.Printf("  container: %s\n", w)
					}
					for _, w := range warnVolumes {
						fmt.Printf("  volume: %s\n", w)
					}
					for _, w := range warnImages {
						fmt.Printf("  image: %s\n", w)
					}
				}
				return nil
			}

			// rm a specific TARGET among managed containers (ignores --workdir).
			if len(args) == 1 {
				inst, err := resolveTarget(d, args[0])
				if err != nil {
					return err
				}
				_ = d.Stop(inst.Container)
				_ = d.Remove(inst.Container)
				fmt.Printf("OK: removed container %q.\n", inst.Container)
				if removeVolume {
					if strings.TrimSpace(inst.Volume) == "" {
						return fmt.Errorf("cannot remove volume: missing %s label on container %q", LabelVolume, inst.Container)
					}
					_ = d.RemoveVolume(inst.Volume)
					fmt.Printf("OK: removed volume %q.\n", inst.Volume)
				}
				return nil
			}

			// Default: rm the instance derived from --workdir (or cwd).
			workdir, _, container, _, volume, err := resolveInstance(cfg)
			if err != nil {
				return err
			}
			if !d.ContainerExists(container) {
				fmt.Printf("OK: no container for workdir: %s\n", workdir)
				return nil
			}
			if err := requireManaged(d, container, workdir); err != nil {
				return err
			}
			_ = d.Stop(container)
			_ = d.Remove(container)
			fmt.Printf("OK: removed container %q.\n", container)
			if removeVolume {
				_ = d.RemoveVolume(volume)
				fmt.Printf("OK: removed volume %q.\n", volume)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&removeVolume, "volume", false, "Also remove the associated /root volume")
	cmd.Flags().BoolVar(&nuke, "nuke", false, "Remove ALL ai-shell managed containers, their volumes, and images they use (destructive)")
	cmd.Flags().BoolVar(&yes, "yes", false, "Skip confirmation prompt (use with --nuke in scripts)")
	return cmd
}

func firstNonEmpty(vals ...string) string {
	for _, v := range vals {
		if strings.TrimSpace(v) != "" {
			return v
		}
	}
	return ""
}

func ternary[T any](cond bool, a, b T) T {
	if cond {
		return a
	}
	return b
}

func newConfigCmd() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Manage ai-shell configuration",
	}

	loadOrDefault := func() (AppConfig, error) {
		cfg, err := readConfigLoose()
		if err == nil {
			return cfg, nil
		}
		// If missing, return a default-initialized config (mode may remain empty).
		if strings.Contains(err.Error(), "not found") {
			return AppConfig{
				DefaultBaseImage: "python:3.12-slim",
				BaseImageAliases: map[string]string{},
			}, nil
		}
		return AppConfig{}, err
	}

	setModeCmd := &cobra.Command{
		Use:   "set-mode <docker|podman>",
		Short: "Set the containerization mode",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			mode := strings.TrimSpace(args[0])
			if err := validateMode(mode); err != nil {
				return err
			}
			cfg, err := loadOrDefault()
			if err != nil {
				return err
			}
			cfg.Mode = mode
			if err := writeConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("OK: configured ai-shell to use %s\n", mode)
			fmt.Printf("Config file: %s\n", getConfigPath())
			return nil
		},
	}

	setDefaultBaseImageCmd := &cobra.Command{
		Use:   "set-default-base-image <image|alias>",
		Short: "Set the default base image for builds (Dockerfile FROM)",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			val := strings.TrimSpace(args[0])
			if err := validateNonEmptyImageRef(val); err != nil {
				return err
			}
			cfg, err := loadOrDefault()
			if err != nil {
				return err
			}
			cfg.DefaultBaseImage = val
			if err := writeConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("OK: default base image set to %s\n", val)
			return nil
		},
	}

	aliasCmd := &cobra.Command{
		Use:   "alias",
		Short: "Manage base image aliases",
	}

	aliasSetCmd := &cobra.Command{
		Use:   "set <alias> <image>",
		Short: "Set an alias for a base image",
		Args:  cobra.ExactArgs(2),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			val := strings.TrimSpace(args[1])
			if !aliasKeyRE.MatchString(key) {
				return fmt.Errorf("invalid alias %q: must match %s", key, aliasKeyRE.String())
			}
			if err := validateNonEmptyImageRef(val); err != nil {
				return err
			}
			cfg, err := loadOrDefault()
			if err != nil {
				return err
			}
			if cfg.BaseImageAliases == nil {
				cfg.BaseImageAliases = map[string]string{}
			}
			cfg.BaseImageAliases[key] = val
			if err := writeConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("OK: alias %s=%s\n", key, val)
			return nil
		},
	}

	aliasRmCmd := &cobra.Command{
		Use:   "rm <alias>",
		Short: "Remove an alias",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			key := strings.TrimSpace(args[0])
			cfg, err := loadOrDefault()
			if err != nil {
				return err
			}
			if cfg.BaseImageAliases == nil {
				cfg.BaseImageAliases = map[string]string{}
			}
			if _, ok := cfg.BaseImageAliases[key]; !ok {
				return fmt.Errorf("alias not found: %s", key)
			}
			delete(cfg.BaseImageAliases, key)
			if err := writeConfig(cfg); err != nil {
				return err
			}
			fmt.Printf("OK: removed alias %s\n", key)
			return nil
		},
	}

	aliasLsCmd := &cobra.Command{
		Use:   "ls",
		Short: "List aliases",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrDefault()
			if err != nil {
				return err
			}
			if len(cfg.BaseImageAliases) == 0 {
				fmt.Println("(no aliases)")
				return nil
			}
			var keys []string
			for k := range cfg.BaseImageAliases {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, k := range keys {
				fmt.Printf("%s=%s\n", k, cfg.BaseImageAliases[k])
			}
			return nil
		},
	}

	showCmd := &cobra.Command{
		Use:   "show",
		Short: "Show current configuration",
		RunE: func(cmd *cobra.Command, args []string) error {
			cfg, err := loadOrDefault()
			if err != nil {
				return err
			}
			fmt.Printf("config:             %s\n", getConfigPath())
			fmt.Printf("mode:               %s\n", strings.TrimSpace(cfg.Mode))
			fmt.Printf("default base image: %s\n", strings.TrimSpace(cfg.DefaultBaseImage))
			fmt.Printf("aliases:            %d\n", len(cfg.BaseImageAliases))
			return nil
		},
	}

	aliasCmd.AddCommand(aliasSetCmd, aliasRmCmd, aliasLsCmd)
	cmd.AddCommand(setModeCmd, setDefaultBaseImageCmd, aliasCmd, showCmd)
	return cmd
}
