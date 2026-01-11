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

func candidateGlobalEnvPaths() []string {
	var out []string
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		out = append(out, filepath.Join(expandUser(xdg), "ai-shell", ".env"))
	}
	out = append(out, expandUser("~/.config/ai-shell/.env"))
	return out
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
	cands := candidateGlobalEnvPaths()
	for i, cand := range cands {
		abs, err := filepath.Abs(cand)
		if err != nil {
			continue
		}
		if _, err := os.Stat(abs); err == nil {
			source := "home"
			if i == 0 && strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")) != "" {
				source = "xdg"
			}
			return EnvFileResolution{
				Path:   abs,
				Args:   []string{"--env-file", abs},
				Source: source,
			}, nil
		}
	}

	return EnvFileResolution{Source: "none"}, nil
}

func defaultGlobalEnvFilePath() string {
	cands := candidateGlobalEnvPaths()
	if len(cands) == 0 {
		return "~/.config/ai-shell/.env"
	}
	// Prefer first candidate (XDG if set, otherwise ~/.config).
	if abs, err := filepath.Abs(cands[0]); err == nil {
		return abs
	}
	return cands[0]
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
// (independent from env-file resolution).
func getConfigDir() string {
	// Config is a stable app preference, not tied to an optional env file.
	if xdg := strings.TrimSpace(os.Getenv("XDG_CONFIG_HOME")); xdg != "" {
		return filepath.Join(expandUser(xdg), "ai-shell")
	}
	return expandUser("~/.config/ai-shell")
}

// getConfigPath returns the full path to the config file
func getConfigPath() string {
	return filepath.Join(getConfigDir(), "config.toml")
}
