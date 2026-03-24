package aishell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInit_ResolvesAliasInDockerfile(t *testing.T) {
	tmpDir, err := os.MkdirTemp("", "ai-shell-init-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	workdir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("failed to create workdir: %v", err)
	}

	cfg := AppConfig{
		Mode:             "docker",
		DefaultBaseImage: "ubu",
		BaseImageAliases: map[string]AliasEntry{
			"ubu": {Image: "ubuntu:24.04", Family: "apt"},
			"deb": {Image: "debian:12-slim", Family: "apt"},
			"fed": {Image: "fedora:40", Family: "dnf"},
		},
	}

	tests := []struct {
		name             string
		aliasKey         string
		wantInFile       string
		wantNotInFile    string
		wantInCompose    string
		wantNotInCompose string
		wantFamily       string
	}{
		{
			name:             "alias deb resolves to debian:12-slim",
			aliasKey:         "deb",
			wantInFile:       "ARG BASE_IMAGE=debian:12-slim",
			wantNotInFile:    "ARG BASE_IMAGE=deb\n",
			wantInCompose:    "BASE_IMAGE=${BASE_IMAGE:-debian:12-slim}",
			wantNotInCompose: "BASE_IMAGE=${BASE_IMAGE:-ubuntu:24.04}",
			wantFamily:       "apt",
		},
		{
			name:             "alias fed resolves to fedora:40",
			aliasKey:         "fed",
			wantInFile:       "ARG BASE_IMAGE=fedora:40",
			wantNotInFile:    "ARG BASE_IMAGE=fed\n",
			wantInCompose:    "BASE_IMAGE=${BASE_IMAGE:-fedora:40}",
			wantNotInCompose: "BASE_IMAGE=${BASE_IMAGE:-ubuntu:24.04}",
			wantFamily:       "dnf",
		},
		{
			name:             "default from config resolves alias",
			aliasKey:         "",
			wantInFile:       "ARG BASE_IMAGE=ubuntu:24.04",
			wantNotInFile:    "",
			wantInCompose:    "BASE_IMAGE=${BASE_IMAGE:-ubuntu:24.04}",
			wantNotInCompose: "",
			wantFamily:       "apt",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			aiShellDir := filepath.Join(workdir, ".ai-shell")
			os.RemoveAll(aiShellDir)

			aliasKey := tt.aliasKey
			if aliasKey == "" {
				aliasKey = cfg.DefaultBaseImage
			}
			resolvedImage, family, err := resolveBaseImage(aliasKey, cfg)
			if err != nil {
				t.Fatalf("failed to resolve base image: %v", err)
			}
			if family != tt.wantFamily {
				t.Fatalf("expected family=%q, got %q", tt.wantFamily, family)
			}

			cliCfg := &Config{Workdir: workdir}
			if err := exportFiles(aiShellDir, workdir, cliCfg, resolvedImage, family, true); err != nil {
				t.Fatalf("exportFiles failed: %v", err)
			}

			// Check Dockerfile
			content, err := os.ReadFile(filepath.Join(aiShellDir, "Dockerfile"))
			if err != nil {
				t.Fatalf("failed to read Dockerfile: %v", err)
			}
			if !strings.Contains(string(content), tt.wantInFile) {
				t.Errorf("Dockerfile should contain %q, got:\n%s", tt.wantInFile, content)
			}
			if tt.wantNotInFile != "" && strings.Contains(string(content), tt.wantNotInFile) {
				t.Errorf("Dockerfile should NOT contain %q, got:\n%s", tt.wantNotInFile, content)
			}

			// Verify markers
			if !strings.Contains(string(content), markerOpen) {
				t.Errorf("Dockerfile should contain opening marker")
			}
			if !strings.Contains(string(content), markerClose) {
				t.Errorf("Dockerfile should contain closing marker")
			}

			// Verify bootstrap-tools.py is NOT written
			if _, err := os.Stat(filepath.Join(aiShellDir, "bootstrap-tools.py")); err == nil {
				t.Errorf("bootstrap-tools.py should not be written")
			}

			// Verify bootstrap-tools.sh IS written
			if _, err := os.Stat(filepath.Join(aiShellDir, "bootstrap-tools.sh")); err != nil {
				t.Errorf("bootstrap-tools.sh should be written: %v", err)
			}

			// Check docker-compose.yml
			composeContent, err := os.ReadFile(filepath.Join(aiShellDir, "docker-compose.yml"))
			if err != nil {
				t.Fatalf("failed to read docker-compose.yml: %v", err)
			}
			if tt.wantInCompose != "" && !strings.Contains(string(composeContent), tt.wantInCompose) {
				t.Errorf("docker-compose.yml should contain %q, got:\n%s", tt.wantInCompose, composeContent)
			}
			if tt.wantNotInCompose != "" && strings.Contains(string(composeContent), tt.wantNotInCompose) {
				t.Errorf("docker-compose.yml should NOT contain %q, got:\n%s", tt.wantNotInCompose, composeContent)
			}
		})
	}
}

func TestSpliceDockerfile(t *testing.T) {
	t.Run("preserves user customizations", func(t *testing.T) {
		existing := markerOpen + "\n" +
			"ARG BASE_IMAGE=ubuntu:24.04\nFROM ${BASE_IMAGE}\n" +
			markerClose + "\n" +
			"\nRUN pip install numpy\n"

		newAuto := markerOpen + "\n" +
			"ARG BASE_IMAGE=debian:12-slim\nFROM ${BASE_IMAGE}\n" +
			markerClose + "\n"

		result, err := spliceDockerfile(existing, newAuto)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(result, "debian:12-slim") {
			t.Errorf("should contain new auto section")
		}
		if !strings.Contains(result, "pip install numpy") {
			t.Errorf("should preserve user customizations")
		}
		if strings.Contains(result, "ubuntu:24.04") {
			t.Errorf("should not contain old auto section")
		}
	})

	t.Run("error on missing markers", func(t *testing.T) {
		_, err := spliceDockerfile("FROM ubuntu:24.04\n", "new content")
		if err == nil {
			t.Fatalf("expected error for missing markers, got nil")
		}
	})

	t.Run("preserves content before opening marker", func(t *testing.T) {
		existing := "# My header comment\n" +
			markerOpen + "\n" +
			"ARG BASE_IMAGE=ubuntu:24.04\n" +
			markerClose + "\n" +
			"\nRUN custom stuff\n"

		newAuto := markerOpen + "\n" +
			"ARG BASE_IMAGE=alpine:3.19\n" +
			markerClose + "\n"

		result, err := spliceDockerfile(existing, newAuto)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasPrefix(result, "# My header comment\n") {
			t.Errorf("should preserve content before marker")
		}
		if !strings.Contains(result, "alpine:3.19") {
			t.Errorf("should contain new auto section")
		}
		if !strings.Contains(result, "custom stuff") {
			t.Errorf("should preserve content after marker")
		}
	})
}

func TestGenerateDockerfile_PerFamily(t *testing.T) {
	families := []struct {
		family    string
		baseImage string
		mustHave  string
	}{
		{"apt", "ubuntu:24.04", "apt-get"},
		{"dnf", "fedora:40", "dnf"},
		{"zypper", "opensuse/leap:15.6", "zypper"},
		{"pacman", "archlinux:latest", "pacman"},
	}
	for _, tt := range families {
		t.Run(tt.family, func(t *testing.T) {
			result, err := generateDockerfile(tt.baseImage, tt.family)
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if !strings.Contains(result, "ARG BASE_IMAGE="+tt.baseImage) {
				t.Errorf("should contain ARG BASE_IMAGE=%s", tt.baseImage)
			}
			if !strings.Contains(result, tt.mustHave) {
				t.Errorf("should contain %q for family %s", tt.mustHave, tt.family)
			}
			if !strings.Contains(result, markerOpen) {
				t.Errorf("should contain opening marker")
			}
			if !strings.Contains(result, markerClose) {
				t.Errorf("should contain closing marker")
			}
		})
	}

	t.Run("unknown family errors", func(t *testing.T) {
		_, err := generateDockerfile("ubuntu:24.04", "yum")
		if err == nil {
			t.Fatalf("expected error for unknown family, got nil")
		}
	})
}
