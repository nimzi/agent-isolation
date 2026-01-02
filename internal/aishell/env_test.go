package aishell

import (
	"os"
	"path/filepath"
	"testing"
)

func withCWD(t *testing.T, dir string) {
	t.Helper()
	old, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	if err := os.Chdir(dir); err != nil {
		t.Fatalf("chdir: %v", err)
	}
	t.Cleanup(func() { _ = os.Chdir(old) })
}

func writeFile(t *testing.T, path string, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("mkdirall: %v", err)
	}
	if err := os.WriteFile(path, []byte(content), 0o600); err != nil {
		t.Fatalf("writefile: %v", err)
	}
}

func TestResolveEnvFileArgs(t *testing.T) {
	t.Run("explicit --env-file missing => error", func(t *testing.T) {
		dir := t.TempDir()
		withCWD(t, dir)
		t.Setenv("AI_SHELL_ENV_FILE", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", dir)

		_, err := resolveEnvFileArgs("missing.env", true)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("explicit --env-file empty => disabled", func(t *testing.T) {
		dir := t.TempDir()
		withCWD(t, dir)
		t.Setenv("AI_SHELL_ENV_FILE", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", dir)

		res, err := resolveEnvFileArgs("", true)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if res.Source != "disabled" {
			t.Fatalf("expected Source=disabled, got %q", res.Source)
		}
		if len(res.Args) != 0 {
			t.Fatalf("expected no args, got %v", res.Args)
		}
		if res.Path != "" {
			t.Fatalf("expected empty path, got %q", res.Path)
		}
	})

	t.Run("AI_SHELL_ENV_FILE missing => error", func(t *testing.T) {
		dir := t.TempDir()
		withCWD(t, dir)
		t.Setenv("AI_SHELL_ENV_FILE", "missing.env")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", dir)

		_, err := resolveEnvFileArgs("", false)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("default XDG path found => xdg", func(t *testing.T) {
		dir := t.TempDir()
		withCWD(t, dir)
		t.Setenv("AI_SHELL_ENV_FILE", "")

		xdg := filepath.Join(dir, "xdg")
		t.Setenv("XDG_CONFIG_HOME", xdg)
		t.Setenv("HOME", filepath.Join(dir, "home"))

		envPath := filepath.Join(xdg, "ai-shell", ".env")
		writeFile(t, envPath, "GH_TOKEN=github_pat_test\n")

		res, err := resolveEnvFileArgs("", false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if res.Source != "xdg" {
			t.Fatalf("expected Source=xdg, got %q", res.Source)
		}
		if res.Path == "" || !filepath.IsAbs(res.Path) {
			t.Fatalf("expected absolute Path, got %q", res.Path)
		}
		if len(res.Args) != 2 || res.Args[0] != "--env-file" || res.Args[1] != res.Path {
			t.Fatalf("unexpected args: %v", res.Args)
		}
	})

	t.Run("default HOME path found => home", func(t *testing.T) {
		dir := t.TempDir()
		withCWD(t, dir)
		t.Setenv("AI_SHELL_ENV_FILE", "")
		t.Setenv("XDG_CONFIG_HOME", "")

		home := filepath.Join(dir, "home")
		t.Setenv("HOME", home)

		envPath := filepath.Join(home, ".config", "ai-shell", ".env")
		writeFile(t, envPath, "GH_TOKEN=github_pat_test\n")

		res, err := resolveEnvFileArgs("", false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if res.Source != "home" {
			t.Fatalf("expected Source=home, got %q", res.Source)
		}
		if res.Path == "" || !filepath.IsAbs(res.Path) {
			t.Fatalf("expected absolute Path, got %q", res.Path)
		}
		if len(res.Args) != 2 || res.Args[0] != "--env-file" || res.Args[1] != res.Path {
			t.Fatalf("unexpected args: %v", res.Args)
		}
	})

	t.Run("none found => none", func(t *testing.T) {
		dir := t.TempDir()
		withCWD(t, dir)
		t.Setenv("AI_SHELL_ENV_FILE", "")
		t.Setenv("XDG_CONFIG_HOME", "")
		t.Setenv("HOME", filepath.Join(dir, "home"))

		res, err := resolveEnvFileArgs("", false)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if res.Source != "none" {
			t.Fatalf("expected Source=none, got %q", res.Source)
		}
		if len(res.Args) != 0 {
			t.Fatalf("expected no args, got %v", res.Args)
		}
		if res.Path != "" {
			t.Fatalf("expected empty path, got %q", res.Path)
		}
	})
}
