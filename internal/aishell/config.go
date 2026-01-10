package aishell

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

const (
	ModeDocker = "docker"
	ModePodman = "podman"
)

type AppConfig struct {
	Mode             string            `json:"mode"`
	DefaultBaseImage string            `json:"defaultBaseImage"`
	BaseImageAliases map[string]string `json:"baseImageAliases"`
}

var aliasKeyRE = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9_-]*$`)

// validateMode validates that mode is either "docker" or "podman"
func validateMode(mode string) error {
	mode = strings.TrimSpace(mode)
	if mode != ModeDocker && mode != ModePodman {
		return fmt.Errorf("invalid mode %q: must be %q or %q", mode, ModeDocker, ModePodman)
	}
	return nil
}

func validateNonEmptyImageRef(s string) error {
	s = strings.TrimSpace(s)
	if s == "" {
		return errors.New("image reference must not be empty")
	}
	// Keep validation conservative: disallow whitespace/newlines which break CLI args.
	if len(strings.Fields(s)) != 1 {
		return fmt.Errorf("invalid image reference %q: must not contain whitespace", s)
	}
	return nil
}

func validateAliases(m map[string]string) error {
	for k, v := range m {
		k = strings.TrimSpace(k)
		v = strings.TrimSpace(v)
		if k == "" || !aliasKeyRE.MatchString(k) {
			return fmt.Errorf("invalid alias key %q: must match %s", k, aliasKeyRE.String())
		}
		if err := validateNonEmptyImageRef(v); err != nil {
			return fmt.Errorf("invalid alias value for %q: %w", k, err)
		}
	}
	return nil
}

func normalizeConfig(cfg AppConfig) AppConfig {
	cfg.Mode = strings.TrimSpace(cfg.Mode)
	cfg.DefaultBaseImage = strings.TrimSpace(cfg.DefaultBaseImage)
	if cfg.BaseImageAliases == nil {
		cfg.BaseImageAliases = map[string]string{}
	}
	return cfg
}

func validateConfig(cfg AppConfig) error {
	if err := validateMode(cfg.Mode); err != nil {
		return err
	}
	if err := validateNonEmptyImageRef(cfg.DefaultBaseImage); err != nil {
		return fmt.Errorf("defaultBaseImage: %w", err)
	}
	if err := validateAliases(cfg.BaseImageAliases); err != nil {
		return err
	}
	return nil
}

// ensureConfigDir ensures the config directory exists
func ensureConfigDir() error {
	dir := getConfigDir()
	return os.MkdirAll(dir, 0o755)
}

// readConfig reads the config file and returns the parsed config.
// Returns an error if the file doesn't exist or is invalid.
func readConfig() (AppConfig, error) {
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return AppConfig{}, fmt.Errorf("config file not found: %s", configPath)
		}
		return AppConfig{}, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("failed to parse config JSON %s: %w", configPath, err)
	}
	cfg = normalizeConfig(cfg)
	if err := validateConfig(cfg); err != nil {
		return AppConfig{}, fmt.Errorf("invalid config %s: %w", configPath, err)
	}
	return cfg, nil
}

// readConfigLoose reads config.json but allows Mode to be empty.
// This is intended for config subcommands that can operate before mode is set.
func readConfigLoose() (AppConfig, error) {
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return AppConfig{}, fmt.Errorf("config file not found: %s", configPath)
		}
		return AppConfig{}, fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}
	var cfg AppConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return AppConfig{}, fmt.Errorf("failed to parse config JSON %s: %w", configPath, err)
	}
	cfg = normalizeConfig(cfg)
	if strings.TrimSpace(cfg.DefaultBaseImage) == "" {
		cfg.DefaultBaseImage = "python:3.12-slim"
	}
	if strings.TrimSpace(cfg.Mode) != "" {
		if err := validateMode(cfg.Mode); err != nil {
			return AppConfig{}, fmt.Errorf("invalid config %s: %w", configPath, err)
		}
	}
	if err := validateNonEmptyImageRef(cfg.DefaultBaseImage); err != nil {
		return AppConfig{}, fmt.Errorf("invalid config %s: defaultBaseImage: %w", configPath, err)
	}
	if err := validateAliases(cfg.BaseImageAliases); err != nil {
		return AppConfig{}, fmt.Errorf("invalid config %s: %w", configPath, err)
	}
	return cfg, nil
}

func writeConfig(cfg AppConfig) error {
	cfg = normalizeConfig(cfg)
	// For writes via config subcommands, ensure aliases map exists.
	if cfg.BaseImageAliases == nil {
		cfg.BaseImageAliases = map[string]string{}
	}
	// If mode is empty (e.g. user only set aliases), allow writing, but keep JSON valid.
	// Commands that require mode will enforce it via ensureConfig/validateConfig.
	if strings.TrimSpace(cfg.Mode) != "" {
		if err := validateMode(cfg.Mode); err != nil {
			return err
		}
	}
	// DefaultBaseImage is required for builds; keep it non-empty on disk.
	if strings.TrimSpace(cfg.DefaultBaseImage) == "" {
		cfg.DefaultBaseImage = "python:3.12-slim"
	}
	if err := validateNonEmptyImageRef(cfg.DefaultBaseImage); err != nil {
		return fmt.Errorf("defaultBaseImage: %w", err)
	}
	if err := validateAliases(cfg.BaseImageAliases); err != nil {
		return err
	}
	if err := ensureConfigDir(); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := getConfigPath()
	b, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config JSON: %w", err)
	}
	b = append(b, '\n')
	if err := os.WriteFile(configPath, b, 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

// ensureConfig ensures the config file exists, prompting interactively if needed
// Returns the config or an error.
func ensureConfig() (AppConfig, error) {
	cfg, err := readConfig()
	if err == nil && strings.TrimSpace(cfg.Mode) != "" {
		return cfg, nil
	}

	// If config exists but is invalid/missing required fields, treat as not configured.
	if !isTTY() {
		configPath := getConfigPath()
		return AppConfig{}, fmt.Errorf(`ai-shell is not configured (missing or invalid config): %s

Please run:
  ai-shell config set-mode <docker|podman>

And optionally set defaults/aliases:
  ai-shell config set-default-base-image <image>
  ai-shell config alias set <alias> <image>

Config directory:
  %s`, configPath, getConfigDir())
	}

	reader := bufio.NewReader(os.Stdin)

	// Interactive prompt: mode
	fmt.Fprintln(os.Stderr, "ai-shell has not been configured. Please select a containerization mode:")
	fmt.Fprintln(os.Stderr, "1) docker")
	fmt.Fprintln(os.Stderr, "2) podman")
	fmt.Fprint(os.Stderr, "\nEnter choice (1-2): ")
	line, err := reader.ReadString('\n')
	if err != nil {
		return AppConfig{}, fmt.Errorf("failed to read input: %w", err)
	}
	choice := strings.TrimSpace(line)
	var selectedMode string
	switch choice {
	case "1":
		selectedMode = ModeDocker
	case "2":
		selectedMode = ModePodman
	default:
		return AppConfig{}, fmt.Errorf("invalid choice %q: must be 1 or 2", choice)
	}

	// Interactive prompt: default base image
	def := "python:3.12-slim"
	fmt.Fprintf(os.Stderr, "\nDefault base image for builds (press Enter for %q): ", def)
	line, err = reader.ReadString('\n')
	if err != nil {
		return AppConfig{}, fmt.Errorf("failed to read input: %w", err)
	}
	base := strings.TrimSpace(line)
	if base == "" {
		base = def
	}
	if err := validateNonEmptyImageRef(base); err != nil {
		return AppConfig{}, fmt.Errorf("invalid default base image: %w", err)
	}

	// Preserve any existing alias map if we managed to parse it, otherwise start empty.
	if cfg.BaseImageAliases == nil {
		cfg.BaseImageAliases = map[string]string{}
	}
	cfg.Mode = selectedMode
	cfg.DefaultBaseImage = base
	cfg = normalizeConfig(cfg)

	if err := writeConfig(cfg); err != nil {
		return AppConfig{}, fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "OK: configured ai-shell (mode=%s, defaultBaseImage=%s)\n", selectedMode, base)
	fmt.Fprintf(os.Stderr, "Config file: %s\n", filepath.Clean(getConfigPath()))
	return cfg, nil
}

// requireRoot checks if the current process is running as root
func requireRoot() error {
	if os.Geteuid() != 0 {
		return errors.New("this command requires root privileges (run with sudo)")
	}
	return nil
}
