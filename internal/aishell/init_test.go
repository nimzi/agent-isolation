package aishell

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestRunInit_ResolvesAliasInDockerfile(t *testing.T) {
	// Create a temporary directory for the test
	tmpDir, err := os.MkdirTemp("", "ai-shell-init-test-*")
	if err != nil {
		t.Fatalf("failed to create temp dir: %v", err)
	}
	defer os.RemoveAll(tmpDir)

	// Create a temporary config file with an alias
	configDir := filepath.Join(tmpDir, "config")
	if err := os.MkdirAll(configDir, 0o755); err != nil {
		t.Fatalf("failed to create config dir: %v", err)
	}

	configContent := `mode = "docker"
default-base-image = "ubu"

[base-image-aliases]
ubu = "ubuntu:24.04"
alp = "alpine:3.19"
`
	configPath := filepath.Join(configDir, "config.toml")
	if err := os.WriteFile(configPath, []byte(configContent), 0o600); err != nil {
		t.Fatalf("failed to write config: %v", err)
	}

	// Create workdir
	workdir := filepath.Join(tmpDir, "project")
	if err := os.MkdirAll(workdir, 0o755); err != nil {
		t.Fatalf("failed to create workdir: %v", err)
	}

	// Create cursor config dir (required by init)
	cursorDir := filepath.Join(tmpDir, "cursor")
	if err := os.MkdirAll(cursorDir, 0o755); err != nil {
		t.Fatalf("failed to create cursor dir: %v", err)
	}

	tests := []struct {
		name          string
		baseImage     string // input (may be alias)
		wantInFile    string // expected resolved image in Dockerfile
		wantNotInFile string // should NOT appear in Dockerfile (use \n to ensure exact line match)
	}{
		{
			name:          "alias deb resolves to debian:12-slim",
			baseImage:     "deb",
			wantInFile:    "ARG BASE_IMAGE=debian:12-slim",
			wantNotInFile: "ARG BASE_IMAGE=deb\n", // exact line - alias alone should not appear
		},
		{
			name:          "alias alp resolves to alpine:3.19",
			baseImage:     "alp",
			wantInFile:    "ARG BASE_IMAGE=alpine:3.19",
			wantNotInFile: "ARG BASE_IMAGE=alp\n",
		},
		{
			name:          "literal image stays as-is",
			baseImage:     "python:3.12-slim",
			wantInFile:    "ARG BASE_IMAGE=python:3.12-slim",
			wantNotInFile: "",
		},
		{
			name:          "default from config resolves alias",
			baseImage:     "", // empty means use default from config (ubu -> ubuntu:24.04)
			wantInFile:    "ARG BASE_IMAGE=ubuntu:24.04",
			wantNotInFile: "", // can't easily test "ubu" since it's substring of "ubuntu"
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Clean up .ai-shell from previous run
			aiShellDir := filepath.Join(workdir, ".ai-shell")
			os.RemoveAll(aiShellDir)

			// Read config
			cfg := AppConfig{
				Mode:             "docker",
				DefaultBaseImage: "ubu",
				BaseImageAliases: map[string]string{
					"ubu": "ubuntu:24.04",
					"alp": "alpine:3.19",
					"deb": "debian:12-slim",
				},
			}

			// Resolve base image (same logic as runInit)
			baseImage := tt.baseImage
			if baseImage == "" {
				baseImage = cfg.DefaultBaseImage
			}
			if baseImage == "" {
				baseImage = "ubuntu:24.04"
			}
			resolvedImage, _, err := resolveBaseImage(baseImage, cfg)
			if err != nil {
				t.Fatalf("failed to resolve base image: %v", err)
			}

			// Export files
			cliCfg := &Config{Workdir: workdir}
			if err := exportFiles(aiShellDir, workdir, cliCfg, resolvedImage, cursorDir, true); err != nil {
				t.Fatalf("exportFiles failed: %v", err)
			}

			// Read the generated Dockerfile
			dockerfilePath := filepath.Join(aiShellDir, "Dockerfile")
			content, err := os.ReadFile(dockerfilePath)
			if err != nil {
				t.Fatalf("failed to read Dockerfile: %v", err)
			}

			// Check that the resolved image is in the file
			if !strings.Contains(string(content), tt.wantInFile) {
				t.Errorf("Dockerfile should contain %q, got:\n%s", tt.wantInFile, content)
			}

			// Check that the alias is NOT in the file (if specified)
			if tt.wantNotInFile != "" && strings.Contains(string(content), tt.wantNotInFile) {
				t.Errorf("Dockerfile should NOT contain %q, got:\n%s", tt.wantNotInFile, content)
			}
		})
	}
}
