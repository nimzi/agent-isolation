package aishell

import (
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
		Short: "Manage per-workdir ai-shell Docker containers",
		Long: strings.TrimSpace(`
Manage per-workdir ai-shell Docker containers.

Workdir is the identity: one container + one /root volume per workdir.

Defaults can be overridden via env vars:
  AI_SHELL_CONTAINER (base name, default: ai-agent-shell)
  AI_SHELL_IMAGE     (default: ai-agent-shell)
  AI_SHELL_VOLUME    (base name, default: ai_agent_shell_home)
`),
		SilenceUsage:  true,
		SilenceErrors: true,
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

	return root
}

func resolveBases(cfg *Config) (containerBase, image, volumeBase string) {
	containerBase = firstNonEmpty(cfg.ContainerBase, os.Getenv("AI_SHELL_CONTAINER"), DefaultContainerBase)
	image = firstNonEmpty(cfg.Image, os.Getenv("AI_SHELL_IMAGE"), DefaultImage)
	volumeBase = firstNonEmpty(cfg.VolumeBase, os.Getenv("AI_SHELL_VOLUME"), DefaultVolumeBase)
	return containerBase, image, volumeBase
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

func resolveEnvFileArg(home string, envFile string, envFileChanged bool) ([]string, error) {
	// If explicitly set to empty, disable.
	if envFileChanged {
		if envFile == "" {
			return nil, nil
		}
		path := envFile
		if !filepath.IsAbs(path) {
			path = filepath.Join(home, path)
		}
		if _, err := os.Stat(path); err != nil {
			return nil, fmt.Errorf("env file not found: %s", envFile)
		}
		return []string{"--env-file", path}, nil
	}
	// Default: use .env if present.
	if _, err := os.Stat(filepath.Join(home, ".env")); err == nil {
		return []string{"--env-file", filepath.Join(home, ".env")}, nil
	}
	return nil, nil
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
	var recreate bool

	use := "up"
	short := "Create/start the workdir container (optionally build/install)"
	if aliasRecreate {
		use = "recreate"
		short = "Alias for: up --recreate"
		recreate = true
	}

	cmd := &cobra.Command{
		Use:   use,
		Short: short,
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
			d := Docker{Dir: dockerDir}
			if err := d.Require(); err != nil {
				return fmt.Errorf("docker not available: %w", err)
			}

			workdir, iid, container, image, volume, err := resolveInstance(cfg)
			if err != nil {
				return err
			}

			cursorDir, err := ensureCursorConfigDir(cursorConfig)
			if err != nil {
				return err
			}

			envArgs, err := resolveEnvFileArg(home, envFile, cmd.Flags().Changed("env-file"))
			if err != nil {
				return err
			}

			if recreate && d.ContainerExists(container) {
				_ = d.Stop(container)
				_ = d.Remove(container)
			}

			if !noBuild {
				if err := d.BuildImage(image); err != nil {
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
					"-v", cursorDir+":/root/.config/cursor",
				)
				args = append(args, envArgs...)
				args = append(args, image)

				if err := d.RunDetached(args...); err != nil {
					return err
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
	cmd.Flags().StringVar(&envFile, "env-file", "", "Env file to pass to docker run (default: use ./.env only if present). Set empty to disable.")
	cmd.Flags().BoolVar(&noBuild, "no-build", false, "Skip docker build")
	cmd.Flags().BoolVar(&noInstall, "no-install", false, "Skip installing cursor-agent")
	if !aliasRecreate {
		cmd.Flags().BoolVar(&recreate, "recreate", false, "Stop/remove and recreate the container if it already exists")
	}

	return cmd
}

func newStartCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "start",
		Short: "Start the container for this workdir",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := Docker{}
			if err := d.Require(); err != nil {
				return err
			}
			workdir, _, container, _, _, err := resolveInstance(cfg)
			if err != nil {
				return err
			}
			if !d.ContainerExists(container) {
				return fmt.Errorf("container not found for workdir: %s (run: ai-shell up)", workdir)
			}
			if err := requireManaged(d, container, workdir); err != nil {
				return err
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
		Use:   "stop",
		Short: "Stop the container for this workdir",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := Docker{}
			if err := d.Require(); err != nil {
				return err
			}
			workdir, _, container, _, _, err := resolveInstance(cfg)
			if err != nil {
				return err
			}
			if !d.ContainerExists(container) {
				return fmt.Errorf("container not found for workdir: %s", workdir)
			}
			if err := requireManaged(d, container, workdir); err != nil {
				return err
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
		Use:   "status",
		Short: "Show status for this workdir instance",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := Docker{}
			if err := d.Require(); err != nil {
				return err
			}
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
			mounts, _ := d.InspectMounts(container)
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
		Use:   "enter",
		Short: "Enter an interactive shell inside the workdir container",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := Docker{}
			if err := d.Require(); err != nil {
				return err
			}
			workdir, _, container, _, _, err := resolveInstance(cfg)
			if err != nil {
				return err
			}
			if !d.ContainerExists(container) {
				return fmt.Errorf("container not found for workdir: %s (run: ai-shell up)", workdir)
			}
			if err := requireManaged(d, container, workdir); err != nil {
				return err
			}
			if !d.ContainerRunning(container) {
				if err := d.Start(container); err != nil {
					return err
				}
			}
			_, _ = d.ExecCapture(container, `grep -q "\.local/bin" ~/.bashrc 2>/dev/null || echo "export PATH=\"$HOME/.local/bin:$PATH\"" >> ~/.bashrc`)

			tty := isTTY()
			argsDocker := []string{"exec"}
			if tty {
				argsDocker = append(argsDocker, "-it")
			} else {
				fmt.Fprintln(os.Stderr, "Warning: no TTY available; running non-interactive shell.")
			}
			argsDocker = append(argsDocker, container, "bash", "-l")

			// Replace process for better UX (signals/TTY)
			return execReplace("docker", argsDocker)
		},
	}
}

func newCheckCmd(cfg *Config) *cobra.Command {
	return &cobra.Command{
		Use:   "check",
		Short: "Sanity-check cursor-agent + mounts (and optional gh auth)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := Docker{}
			if err := d.Require(); err != nil {
				return err
			}
			workdir, _, container, _, _, err := resolveInstance(cfg)
			if err != nil {
				return err
			}
			if !d.ContainerExists(container) {
				return fmt.Errorf("container not found for workdir: %s (run: ai-shell up)", workdir)
			}
			if err := requireManaged(d, container, workdir); err != nil {
				return err
			}
			if !d.ContainerRunning(container) {
				return fmt.Errorf("container is not running: %s (run: ai-shell start)", container)
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
			d := Docker{}
			if err := d.Require(); err != nil {
				return err
			}
			names, err := d.PSNamesByLabel(LabelManaged, "true")
			if err != nil {
				return err
			}
			if len(names) == 0 {
				fmt.Println("No ai-shell managed containers found.")
				return nil
			}
			sort.Strings(names)

			type row struct {
				workdir   string
				container string
				status    string
			}
			var rows []row
			for _, name := range names {
				info, err := d.InspectContainer(name)
				if err != nil {
					continue
				}
				wd := ""
				if info.Config.Labels != nil {
					wd = info.Config.Labels[LabelWorkdir]
				}
				rows = append(rows, row{workdir: wd, container: name, status: info.State.Status})
			}

			wdW, cW := 6, 9
			for _, r := range rows {
				if len(r.workdir) > wdW {
					wdW = len(r.workdir)
				}
				if len(r.container) > cW {
					cW = len(r.container)
				}
			}
			fmt.Printf("%-*s  %-*s  STATUS\n", wdW, "WORKDIR", cW, "CONTAINER")
			for _, r := range rows {
				fmt.Printf("%-*s  %-*s  %s\n", wdW, r.workdir, cW, r.container, r.status)
			}
			return nil
		},
	}
}

func newRmCmd(cfg *Config) *cobra.Command {
	var removeVolume bool
	cmd := &cobra.Command{
		Use:   "rm",
		Short: "Remove the workdir container (optionally its /root volume)",
		RunE: func(cmd *cobra.Command, args []string) error {
			d := Docker{}
			if err := d.Require(); err != nil {
				return err
			}
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
