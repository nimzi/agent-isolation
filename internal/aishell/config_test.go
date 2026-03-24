package aishell

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConfigReadWriteTOML(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	cfg := AppConfig{
		Mode:             ModeDocker,
		DefaultBaseImage: "py",
		BaseImageAliases: map[string]AliasEntry{
			"u24": {Image: "ubuntu:24.04", Family: "apt"},
			"py":  {Image: "python:3.12-slim", Family: "apt"},
		},
	}
	if err := writeConfig(cfg); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	if _, err := os.Stat(filepath.Join(dir, "ai-shell", "config.toml")); err != nil {
		t.Fatalf("expected config.toml to exist: %v", err)
	}
	got, err := readConfig()
	if err != nil {
		t.Fatalf("readConfig: %v", err)
	}
	if got.Mode != ModeDocker {
		t.Fatalf("expected mode=%q, got %q", ModeDocker, got.Mode)
	}
	if got.DefaultBaseImage != "py" {
		t.Fatalf("expected default-base-image py, got %q", got.DefaultBaseImage)
	}
	entry := got.BaseImageAliases["u24"]
	if entry.Image != "ubuntu:24.04" || entry.Family != "apt" {
		t.Fatalf("expected alias u24={ubuntu:24.04, apt}, got %+v", entry)
	}
}

func TestReadConfigLoose_AllowsMissingMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	if err := writeConfig(AppConfig{
		DefaultBaseImage: "py",
		BaseImageAliases: map[string]AliasEntry{
			"py": {Image: "python:3.12-slim", Family: "apt"},
		},
	}); err != nil {
		t.Fatalf("writeConfig: %v", err)
	}
	if _, err := readConfig(); err == nil {
		t.Fatalf("expected strict readConfig to fail without mode, got nil")
	}
	if _, err := readConfigLoose(); err != nil {
		t.Fatalf("expected readConfigLoose to succeed, got %v", err)
	}
}
