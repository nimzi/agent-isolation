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

	tests := []struct {
		name             string
		baseImage        string // input (may be alias)
		wantInFile       string // expected resolved image in Dockerfile
		wantNotInFile    string // should NOT appear in Dockerfile
		wantInCompose    string // expected in docker-compose.yml build args
		wantNotInCompose string // should NOT appear in docker-compose.yml build args
	}{
		{
			name:               "alias deb resolves to debian:12-slim",
			baseImage:          "deb",
			wantInFile:         "ARG BASE_IMAGE=debian:12-slim",
			wantNotInFile:      "ARG BASE_IMAGE=deb\n",
			wantInCompose:      "BASE_IMAGE=${BASE_IMAGE:-debian:12-slim}",
			wantNotInCompose:   "BASE_IMAGE=${BASE_IMAGE:-ubuntu:24.04}",
		},
		{
			name:               "alias alp resolves to alpine:3.19",
			baseImage:          "alp",
			wantInFile:         "ARG BASE_IMAGE=alpine:3.19",
			wantNotInFile:      "ARG BASE_IMAGE=alp\n",
			wantInCompose:      "BASE_IMAGE=${BASE_IMAGE:-alpine:3.19}",
			wantNotInCompose:   "BASE_IMAGE=${BASE_IMAGE:-ubuntu:24.04}",
		},
		{
			name:               "literal image stays as-is",
			baseImage:          "python:3.12-slim",
			wantInFile:         "ARG BASE_IMAGE=python:3.12-slim",
			wantNotInFile:      "",
			wantInCompose:      "BASE_IMAGE=${BASE_IMAGE:-python:3.12-slim}",
			wantNotInCompose:   "BASE_IMAGE=${BASE_IMAGE:-ubuntu:24.04}",
		},
		{
			name:               "default from config resolves alias",
			baseImage:          "",
			wantInFile:         "ARG BASE_IMAGE=ubuntu:24.04",
			wantNotInFile:      "",
			wantInCompose:      "BASE_IMAGE=${BASE_IMAGE:-ubuntu:24.04}",
			wantNotInCompose:   "",
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
			if err := exportFiles(aiShellDir, workdir, cliCfg, resolvedImage, true); err != nil {
				t.Fatalf("exportFiles failed: %v", err)
			}

			// Read the generated Dockerfile
			dockerfilePath := filepath.Join(aiShellDir, "Dockerfile")
			content, err := os.ReadFile(dockerfilePath)
			if err != nil {
				t.Fatalf("failed to read Dockerfile: %v", err)
			}

			// Check that the resolved image is in the Dockerfile
			if !strings.Contains(string(content), tt.wantInFile) {
				t.Errorf("Dockerfile should contain %q, got:\n%s", tt.wantInFile, content)
			}

			// Check that the alias is NOT in the Dockerfile (if specified)
			if tt.wantNotInFile != "" && strings.Contains(string(content), tt.wantNotInFile) {
				t.Errorf("Dockerfile should NOT contain %q, got:\n%s", tt.wantNotInFile, content)
			}

			// Read the generated docker-compose.yml
			composePath := filepath.Join(aiShellDir, "docker-compose.yml")
			composeContent, err := os.ReadFile(composePath)
			if err != nil {
				t.Fatalf("failed to read docker-compose.yml: %v", err)
			}

			// Check that the resolved image is the build arg default in docker-compose.yml
			if tt.wantInCompose != "" && !strings.Contains(string(composeContent), tt.wantInCompose) {
				t.Errorf("docker-compose.yml should contain %q, got:\n%s", tt.wantInCompose, composeContent)
			}

			// Check that the old default is NOT in docker-compose.yml (if specified)
			if tt.wantNotInCompose != "" && strings.Contains(string(composeContent), tt.wantNotInCompose) {
				t.Errorf("docker-compose.yml should NOT contain %q, got:\n%s", tt.wantNotInCompose, composeContent)
			}
		})
	}
}
