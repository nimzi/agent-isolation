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
		DefaultBaseImage: "python:3.12-slim",
		BaseImageAliases: map[string]string{
			"u24": "ubuntu:24.04",
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
	if got.DefaultBaseImage != "python:3.12-slim" {
		t.Fatalf("expected default-base-image python:3.12-slim, got %q", got.DefaultBaseImage)
	}
	if got.BaseImageAliases["u24"] != "ubuntu:24.04" {
		t.Fatalf("expected alias u24=ubuntu:24.04, got %q", got.BaseImageAliases["u24"])
	}
}

func TestReadConfigLoose_AllowsMissingMode(t *testing.T) {
	dir := t.TempDir()
	t.Setenv("XDG_CONFIG_HOME", dir)
	t.Setenv("HOME", dir)

	// Write a config without mode via the same writer (allowed for config subcommands).
	if err := writeConfig(AppConfig{
		DefaultBaseImage: "python:3.12-slim",
		BaseImageAliases: map[string]string{"py": "python:3.12-slim"},
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
