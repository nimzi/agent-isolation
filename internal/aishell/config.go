package aishell

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"strings"
)

const (
	ModeDocker = "docker"
	ModePodman = "podman"
)

// validateMode validates that mode is either "docker" or "podman"
func validateMode(mode string) error {
	mode = strings.TrimSpace(mode)
	if mode != ModeDocker && mode != ModePodman {
		return fmt.Errorf("invalid mode %q: must be %q or %q", mode, ModeDocker, ModePodman)
	}
	return nil
}

// ensureConfigDir ensures the config directory exists
func ensureConfigDir() error {
	dir := getConfigDir()
	return os.MkdirAll(dir, 0o755)
}

// readConfig reads the config file and returns the mode
// Returns an error if the file doesn't exist or is invalid
func readConfig() (string, error) {
	configPath := getConfigPath()
	data, err := os.ReadFile(configPath)
	if err != nil {
		if os.IsNotExist(err) {
			return "", fmt.Errorf("config file not found: %s", configPath)
		}
		return "", fmt.Errorf("failed to read config file %s: %w", configPath, err)
	}

	content := strings.TrimSpace(string(data))
	lines := strings.Split(content, "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}
		if strings.HasPrefix(line, "mode=") {
			mode := strings.TrimSpace(strings.TrimPrefix(line, "mode="))
			if err := validateMode(mode); err != nil {
				return "", fmt.Errorf("invalid mode in config file: %w", err)
			}
			return mode, nil
		}
	}

	return "", fmt.Errorf("config file %s does not contain a valid mode= setting", configPath)
}

// writeConfig writes the config file with the specified mode
func writeConfig(mode string) error {
	if err := validateMode(mode); err != nil {
		return err
	}

	if err := ensureConfigDir(); err != nil {
		return fmt.Errorf("failed to create config directory: %w", err)
	}

	configPath := getConfigPath()
	content := fmt.Sprintf("mode=%s\n", mode)
	if err := os.WriteFile(configPath, []byte(content), 0o600); err != nil {
		return fmt.Errorf("failed to write config file %s: %w", configPath, err)
	}

	return nil
}

// ensureConfig ensures the config file exists, prompting interactively if needed
// Returns the mode or an error
func ensureConfig() (string, error) {
	mode, err := readConfig()
	if err == nil {
		return mode, nil
	}

	// Check if error is because file doesn't exist
	if !strings.Contains(err.Error(), "not found") {
		return "", err
	}

	// Config file doesn't exist - check if we can prompt interactively
	if !isTTY() {
		configPath := getConfigPath()
		return "", fmt.Errorf(`ai-shell config file not found: %s

Please run:
  ai-shell config set-mode <docker|podman>

The config file must be in the same directory as your .env file:
  %s`, configPath, getConfigDir())
	}

	// Interactive prompt
	fmt.Fprintln(os.Stderr, "ai-shell has not been configured. Please select a containerization mode:")
	fmt.Fprintln(os.Stderr, "1) docker")
	fmt.Fprintln(os.Stderr, "2) podman")
	fmt.Fprint(os.Stderr, "\nEnter choice (1-2): ")

	reader := bufio.NewReader(os.Stdin)
	line, err := reader.ReadString('\n')
	if err != nil {
		return "", fmt.Errorf("failed to read input: %w", err)
	}

	choice := strings.TrimSpace(line)
	var selectedMode string
	switch choice {
	case "1":
		selectedMode = ModeDocker
	case "2":
		selectedMode = ModePodman
	default:
		return "", fmt.Errorf("invalid choice %q: must be 1 or 2", choice)
	}

	if err := writeConfig(selectedMode); err != nil {
		return "", fmt.Errorf("failed to write config: %w", err)
	}

	fmt.Fprintf(os.Stderr, "OK: configured ai-shell to use %s\n", selectedMode)
	return selectedMode, nil
}

// requireRoot checks if the current process is running as root
func requireRoot() error {
	if os.Geteuid() != 0 {
		return errors.New("this command requires root privileges (run with sudo)")
	}
	return nil
}
