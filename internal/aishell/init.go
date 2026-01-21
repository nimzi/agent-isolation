package aishell

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/pelletier/go-toml/v2"
	"golang.org/x/term"
)

// setupOptions for the global 'setup' command (one-time per machine)
type setupOptions struct {
	Yes         bool
	Force       bool
	Mode        string
	ConfigDir   string
	EnvPath     string
	GHTokenCmd  string
	SkipGHToken bool
	Interactive bool
}

// initOptions for the per-project 'init' command
type initOptions struct {
	Force        bool
	Workdir      string
	BaseImage    string
	CursorConfig string
}

func getHostGHToken(cmd string) (string, error) {
	cmd = strings.TrimSpace(cmd)
	if cmd == "" {
		cmd = "gh auth token"
	}
	// Execute as a shell command so users can pass quoted args.
	c := exec.Command("sh", "-c", cmd)
	out, err := c.Output()
	if err != nil {
		// Keep the error without leaking token; output() shouldn't contain it anyway.
		return "", fmt.Errorf("run %q: %w", cmd, err)
	}
	token := strings.TrimSpace(string(out))
	if token == "" {
		return "", fmt.Errorf("%q returned empty output", cmd)
	}
	return token, nil
}

func promptSecret(prompt string) (string, error) {
	if strings.TrimSpace(prompt) == "" {
		prompt = "Enter secret: "
	}
	fmt.Fprint(os.Stderr, prompt)
	b, err := term.ReadPassword(int(os.Stdin.Fd()))
	fmt.Fprintln(os.Stderr) // newline after hidden entry
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(b)), nil
}

func initWriteEnvFile(envPath string, token string, force bool) (bool, error) {
	envPath = filepath.Clean(expandUser(strings.TrimSpace(envPath)))
	if envPath == "" {
		return false, errors.New("env path is empty")
	}
	if !filepath.IsAbs(envPath) {
		abs, err := filepath.Abs(envPath)
		if err != nil {
			return false, err
		}
		envPath = abs
	}

	if !force {
		if _, err := os.Stat(envPath); err == nil {
			return false, fmt.Errorf("refusing to overwrite existing file: %s (use --force)", envPath)
		}
	}

	if err := os.MkdirAll(filepath.Dir(envPath), 0o755); err != nil {
		return false, err
	}

	var b strings.Builder
	b.WriteString("# ai-shell global env file\n")
	b.WriteString("# This file is injected into containers via --env-file.\n")
	b.WriteString("# Recommended permissions: chmod 600 " + envPath + "\n\n")
	if strings.TrimSpace(token) != "" {
		b.WriteString("GH_TOKEN=" + strings.TrimSpace(token) + "\n")
	} else {
		b.WriteString("# GH_TOKEN=github_pat_...\n")
		b.WriteString("# Tip: you can also authenticate interactively inside the container:\n")
		b.WriteString("#   ai-shell enter\n")
		b.WriteString("#   gh auth login\n")
	}
	if err := os.WriteFile(envPath, []byte(b.String()), 0o600); err != nil {
		return false, err
	}
	return strings.TrimSpace(token) != "", nil
}

func initWriteConfigAt(configPath string, cfg AppConfig, force bool) error {
	configPath = filepath.Clean(expandUser(strings.TrimSpace(configPath)))
	if configPath == "" {
		return errors.New("config path is empty")
	}
	if !filepath.IsAbs(configPath) {
		abs, err := filepath.Abs(configPath)
		if err != nil {
			return err
		}
		configPath = abs
	}

	if !force {
		if _, err := os.Stat(configPath); err == nil {
			return fmt.Errorf("refusing to overwrite existing file: %s (use --force)", configPath)
		}
	}
	if err := os.MkdirAll(filepath.Dir(configPath), 0o755); err != nil {
		return err
	}

	// Temporarily override config path by writing directly to configPath.
	// Reuse TOML encoding logic by calling writeConfig in-place is not possible because writeConfig
	// always targets getConfigPath(). So we encode here.
	cfg = normalizeConfig(cfg)
	if cfg.BaseImageAliases == nil {
		cfg.BaseImageAliases = defaultAppConfig().BaseImageAliases
	}
	// Mode may be empty here; validate accordingly.
	if strings.TrimSpace(cfg.Mode) != "" {
		if err := validateMode(cfg.Mode); err != nil {
			return err
		}
	}
	if strings.TrimSpace(cfg.DefaultBaseImage) == "" {
		cfg.DefaultBaseImage = defaultAppConfig().DefaultBaseImage
	}
	if err := validateNonEmptyImageRef(cfg.DefaultBaseImage); err != nil {
		return fmt.Errorf("default-base-image: %w", err)
	}
	if err := validateAliases(cfg.BaseImageAliases); err != nil {
		return err
	}

	// Encode TOML with the existing dependency (go-toml v2) by importing it indirectly via config.go.
	b, err := toml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("failed to encode config TOML: %w", err)
	}
	if len(b) == 0 || b[len(b)-1] != '\n' {
		b = append(b, '\n')
	}
	if err := os.WriteFile(configPath, b, 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}
	return nil
}

func initResolveConfigPath(configDir string) (string, error) {
	if strings.TrimSpace(configDir) == "" {
		return getConfigPath(), nil
	}
	dir := expandUser(strings.TrimSpace(configDir))
	if dir == "" {
		return "", errors.New("config-dir is empty")
	}
	if !filepath.IsAbs(dir) {
		abs, err := filepath.Abs(dir)
		if err != nil {
			return "", err
		}
		dir = abs
	}
	return filepath.Join(dir, "config.toml"), nil
}

func initResolveEnvPath(envPath string) (string, error) {
	if strings.TrimSpace(envPath) != "" {
		p := expandUser(strings.TrimSpace(envPath))
		if !filepath.IsAbs(p) {
			abs, err := filepath.Abs(p)
			if err != nil {
				return "", err
			}
			p = abs
		}
		return p, nil
	}
	cands := candidateGlobalEnvPaths()
	if len(cands) == 0 {
		return "", errors.New("no candidate env paths")
	}
	p := expandUser(strings.TrimSpace(cands[0]))
	if !filepath.IsAbs(p) {
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		p = abs
	}
	return p, nil
}

func promptLine(reader *bufio.Reader, prompt string) (string, error) {
	fmt.Fprint(os.Stderr, prompt)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(line), nil
}

func setupInteractive(reader *bufio.Reader, mode string, configDir string, envPath string, ghTokenCmd string, skipGHToken bool) (resolvedMode string, resolvedConfigPath string, resolvedEnvPath string, resolvedEnvToken string, wroteToken bool, err error) {
	// config dir prompt (default getConfigDir())
	if strings.TrimSpace(configDir) == "" {
		def := getConfigDir()
		line, err := promptLine(reader, fmt.Sprintf("Config dir (press Enter for %q): ", def))
		if err != nil {
			return "", "", "", "", false, err
		}
		if line == "" {
			configDir = def
		} else {
			configDir = line
		}
	}
	resolvedConfigPath, err = initResolveConfigPath(configDir)
	if err != nil {
		return "", "", "", "", false, err
	}

	// env path prompt (default candidateGlobalEnvPaths()[0], allow choosing candidates/custom)
	if strings.TrimSpace(envPath) == "" {
		cands := candidateGlobalEnvPaths()
		fmt.Fprintln(os.Stderr)
		fmt.Fprintln(os.Stderr, "Select a global .env path for GH_TOKEN:")
		for i, p := range cands {
			fmt.Fprintf(os.Stderr, "%d) %s\n", i+1, p)
		}
		customIdx := len(cands) + 1
		fmt.Fprintf(os.Stderr, "%d) Custom path\n", customIdx)
		line, err := promptLine(reader, fmt.Sprintf("\nEnter choice (1-%d): ", customIdx))
		if err != nil {
			return "", "", "", "", false, err
		}
		n, err := strconv.Atoi(strings.TrimSpace(line))
		if err != nil {
			return "", "", "", "", false, errors.New("invalid choice")
		}
		if n >= 1 && n <= len(cands) {
			envPath = cands[n-1]
		} else if n == customIdx {
			p, err := promptLine(reader, "Enter env path: ")
			if err != nil {
				return "", "", "", "", false, err
			}
			envPath = p
		} else {
			return "", "", "", "", false, errors.New("invalid choice")
		}
	}
	resolvedEnvPath, err = initResolveEnvPath(envPath)
	if err != nil {
		return "", "", "", "", false, err
	}

	// mode prompt (only if not already provided)
	resolvedMode = strings.TrimSpace(mode)
	if resolvedMode == "" {
		fmt.Fprintln(os.Stderr, "Select a containerization mode:")
		fmt.Fprintln(os.Stderr, "1) docker")
		fmt.Fprintln(os.Stderr, "2) podman")
		line, err := promptLine(reader, "\nEnter choice (1-2): ")
		if err != nil {
			return "", "", "", "", false, err
		}
		switch line {
		case "1":
			resolvedMode = ModeDocker
		case "2":
			resolvedMode = ModePodman
		default:
			return "", "", "", "", false, errors.New("invalid choice (must be 1 or 2)")
		}
	}
	if err := validateMode(resolvedMode); err != nil {
		return "", "", "", "", false, err
	}

	// GH_TOKEN choice prompt
	if skipGHToken {
		return resolvedMode, resolvedConfigPath, resolvedEnvPath, "", false, nil
	}
	ghTokenCmd = strings.TrimSpace(ghTokenCmd)
	if ghTokenCmd == "" {
		ghTokenCmd = "gh auth token"
	}

	fmt.Fprintln(os.Stderr)
	fmt.Fprintln(os.Stderr, "Configure GH_TOKEN for non-interactive GitHub auth inside the container:")
	fmt.Fprintf(os.Stderr, "1) Run host command (%q)\n", ghTokenCmd)
	fmt.Fprintln(os.Stderr, "2) Enter token manually (hidden)")
	fmt.Fprintln(os.Stderr, "3) Skip")
	line, err := promptLine(reader, "\nEnter choice (1-3): ")
	if err != nil {
		return "", "", "", "", false, err
	}
	switch line {
	case "1":
		tok, err := getHostGHToken(ghTokenCmd)
		if err != nil {
			return "", "", "", "", false, err
		}
		return resolvedMode, resolvedConfigPath, resolvedEnvPath, tok, true, nil
	case "2":
		tok, err := promptSecret("Enter GH_TOKEN (input hidden): ")
		if err != nil {
			return "", "", "", "", false, err
		}
		if strings.TrimSpace(tok) == "" {
			return "", "", "", "", false, errors.New("GH_TOKEN was empty")
		}
		return resolvedMode, resolvedConfigPath, resolvedEnvPath, tok, true, nil
	case "3":
		return resolvedMode, resolvedConfigPath, resolvedEnvPath, "", false, nil
	default:
		return "", "", "", "", false, errors.New("invalid choice (must be 1-3)")
	}
}

func runSetup(opts setupOptions) error {
	interactive := opts.Interactive
	if opts.Yes {
		interactive = false
	}

	configPath := ""
	envPath := ""

	mode := strings.TrimSpace(opts.Mode)
	if !interactive {
		if mode == "" {
			mode = ModeDocker
		}
		if err := validateMode(mode); err != nil {
			return err
		}
	}

	var token string
	var tokenSet bool
	ghCmd := strings.TrimSpace(opts.GHTokenCmd)
	if ghCmd == "" {
		ghCmd = "gh auth token"
	}

	if interactive {
		reader := bufio.NewReader(os.Stdin)
		resolvedMode, resolvedConfigPath, resolvedEnvPath, tok, set, err := setupInteractive(reader, mode, opts.ConfigDir, opts.EnvPath, ghCmd, opts.SkipGHToken)
		if err != nil {
			return err
		}
		mode = resolvedMode
		configPath = resolvedConfigPath
		envPath = resolvedEnvPath
		token = tok
		tokenSet = set
	} else {
		var err error
		configPath, err = initResolveConfigPath(opts.ConfigDir)
		if err != nil {
			return err
		}
		envPath, err = initResolveEnvPath(opts.EnvPath)
		if err != nil {
			return err
		}
		// Non-interactive: best-effort GH_TOKEN retrieval (unless skipped).
		if !opts.SkipGHToken {
			if tok, err := getHostGHToken(ghCmd); err == nil {
				token = tok
				tokenSet = true
			}
		}
	}

	cfg := defaultAppConfig()
	cfg.Mode = mode
	if err := initWriteConfigAt(configPath, cfg, opts.Force); err != nil {
		return err
	}
	wroteToken, err := initWriteEnvFile(envPath, token, opts.Force)
	if err != nil {
		return err
	}
	// If we attempted to set token but ended up empty, wroteToken will be false.
	_ = tokenSet

	fmt.Fprintf(os.Stderr, "OK: initialized global ai-shell config\n")
	fmt.Fprintf(os.Stderr, "config: %s\n", configPath)
	fmt.Fprintf(os.Stderr, "env:    %s\n", envPath)
	fmt.Fprintf(os.Stderr, "mode:   %s\n", mode)
	fmt.Fprintf(os.Stderr, "GH_TOKEN set: %t\n", wroteToken)
	fmt.Fprintf(os.Stderr, "\nNext: run 'ai-shell init' in your project directory to scaffold .ai-shell/\n")

	return nil
}

func runInit(opts initOptions) error {
	// Read existing config to get default base image
	cfg, err := readConfig()
	if err != nil {
		return fmt.Errorf("failed to read config: %w", err)
	}

	workdir, err := CanonicalWorkdir(opts.Workdir)
	if err != nil {
		return fmt.Errorf("failed to resolve workdir: %w", err)
	}

	aiShellDir := filepath.Join(workdir, ".ai-shell")

	// Resolve cursor config directory
	cursorConfig := opts.CursorConfig
	if cursorConfig == "" {
		cursorConfig = "~/.config/cursor"
	}
	cursorDir, err := ensureCursorConfigDir(cursorConfig)
	if err != nil {
		return fmt.Errorf("failed to resolve cursor config: %w", err)
	}

	// Resolve base image (and expand aliases)
	baseImage := opts.BaseImage
	if baseImage == "" {
		baseImage = cfg.DefaultBaseImage
	}
	if baseImage == "" {
		baseImage = "ubuntu:24.04"
	}
	// Resolve alias to actual image reference
	resolvedImage, _, err := resolveBaseImage(baseImage, cfg)
	if err != nil {
		return fmt.Errorf("failed to resolve base image: %w", err)
	}
	baseImage = resolvedImage

	// Use exportFiles to scaffold .ai-shell/
	cliCfg := &Config{Workdir: workdir}
	if err := exportFiles(aiShellDir, workdir, cliCfg, baseImage, cursorDir, opts.Force); err != nil {
		return fmt.Errorf("failed to scaffold .ai-shell/: %w", err)
	}

	fmt.Fprintf(os.Stderr, "OK: initialized workdir .ai-shell/\n")
	fmt.Fprintf(os.Stderr, "workdir: %s\n", workdir)
	fmt.Fprintf(os.Stderr, ".ai-shell: %s\n", aiShellDir)
	fmt.Fprintf(os.Stderr, "\nNext: run 'ai-shell up' to build and start the container\n")

	return nil
}
