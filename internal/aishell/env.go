package aishell

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

type EnvFileResolution struct {
	Path   string
	Args   []string
	Source string // flag, env, xdg, home, none, disabled
}

func resolveEnvFileArgs(flagValue string, flagChanged bool) (EnvFileResolution, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return EnvFileResolution{}, err
	}

	resolvePath := func(p string) (string, error) {
		p = strings.TrimSpace(p)
		p = expandUser(p)
		if p == "" {
			return "", nil
		}
		if !filepath.IsAbs(p) {
			p = filepath.Join(cwd, p)
		}
		abs, err := filepath.Abs(p)
		if err != nil {
			return "", err
		}
		return abs, nil
	}

	mustExist := func(abs string, original string) error {
		if abs == "" {
			return fmt.Errorf("env file path is empty: %s", original)
		}
		if _, err := os.Stat(abs); err != nil {
			return fmt.Errorf("env file not found: %s", original)
		}
		return nil
	}

	if flagChanged {
		// If explicitly set to empty, disable env-file injection.
		if flagValue == "" {
			return EnvFileResolution{Source: "disabled"}, nil
		}
		abs, err := resolvePath(flagValue)
		if err != nil {
			return EnvFileResolution{}, err
		}
		if err := mustExist(abs, flagValue); err != nil {
			return EnvFileResolution{}, err
		}
		return EnvFileResolution{
			Path:   abs,
			Args:   []string{"--env-file", abs},
			Source: "flag",
		}, nil
	}

	if env := strings.TrimSpace(os.Getenv("AI_SHELL_ENV_FILE")); env != "" {
		abs, err := resolvePath(env)
		if err != nil {
			return EnvFileResolution{}, err
		}
		if err := mustExist(abs, env); err != nil {
			return EnvFileResolution{}, err
		}
		return EnvFileResolution{
			Path:   abs,
			Args:   []string{"--env-file", abs},
			Source: "env",
		}, nil
	}

	// Defaults: XDG config first, then ~/.config.
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		cand := filepath.Join(expandUser(xdg), "ai-shell", ".env")
		abs, err := filepath.Abs(cand)
		if err == nil {
			if _, err := os.Stat(abs); err == nil {
				return EnvFileResolution{
					Path:   abs,
					Args:   []string{"--env-file", abs},
					Source: "xdg",
				}, nil
			}
		}
	}

	homeCand := expandUser("~/.config/ai-shell/.env")
	if abs, err := filepath.Abs(homeCand); err == nil {
		if _, err := os.Stat(abs); err == nil {
			return EnvFileResolution{
				Path:   abs,
				Args:   []string{"--env-file", abs},
				Source: "home",
			}, nil
		}
	}

	return EnvFileResolution{Source: "none"}, nil
}

func defaultGlobalEnvFilePath() string {
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		p := filepath.Join(expandUser(xdg), "ai-shell", ".env")
		if abs, err := filepath.Abs(p); err == nil {
			return abs
		}
		return p
	}
	p := expandUser("~/.config/ai-shell/.env")
	if abs, err := filepath.Abs(p); err == nil {
		return abs
	}
	return p
}

func formatEnvMissingWarning(defaultPath string) string {
	defaultPath = strings.TrimSpace(defaultPath)
	if defaultPath == "" {
		defaultPath = "~/.config/ai-shell/.env"
	}
	return strings.TrimSpace(fmt.Sprintf(`
Warning: no env file was provided/found, so gh may not be authenticated non-interactively.

Why this matters:
  - ai-shell uses gh to do first-run GitHub SSH setup (/docker/setup-git-ssh.sh).
  - Without a token, you can still use the container, but SSH bootstrap may be deferred.

Authenticate interactively (recommended if you don't want to store a token):
  - ai-shell enter            (or: ai-shell enter <short>)
  - inside the container: gh auth login
  - optionally verify: gh auth status

Create a global env file (optional, for non-interactive runs):
  - preferred: $XDG_CONFIG_HOME/ai-shell/.env
  - fallback:  ~/.config/ai-shell/.env
  - example contents:
      GH_TOKEN=github_pat_...
  - recommended permissions:
      chmod 600 %s

Finish SSH setup after authenticating:
  - re-run: ai-shell up            (it will attempt SSH setup if gh auth status passes)
  - or inside the container: /docker/setup-git-ssh.sh
`, defaultPath))
}

// getConfigDir returns the directory where the config file should be located
// (same directory as .env file)
func getConfigDir() string {
	envPath := defaultGlobalEnvFilePath()
	return filepath.Dir(envPath)
}

// getConfigPath returns the full path to the config file
func getConfigPath() string {
	return filepath.Join(getConfigDir(), "config")
}
