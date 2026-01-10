package aishell

import "testing"

func TestChooseBaseImage(t *testing.T) {
	t.Run("flag wins and resolves alias", func(t *testing.T) {
		cfg := AppConfig{
			DefaultBaseImage: "python:3.12-slim",
			BaseImageAliases: map[string]string{"u24": "ubuntu:24.04"},
		}
		got, source, aliased, err := chooseBaseImage("u24", nil, cfg)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got != "ubuntu:24.04" || source != "flag" || !aliased {
			t.Fatalf("unexpected: got=%q source=%q aliased=%v", got, source, aliased)
		}
	})

	t.Run("arg used when flag empty", func(t *testing.T) {
		cfg := AppConfig{DefaultBaseImage: "python:3.12-slim"}
		got, source, aliased, err := chooseBaseImage("", []string{"debian:12-slim"}, cfg)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got != "debian:12-slim" || source != "arg" || aliased {
			t.Fatalf("unexpected: got=%q source=%q aliased=%v", got, source, aliased)
		}
	})

	t.Run("default resolves via alias", func(t *testing.T) {
		cfg := AppConfig{
			DefaultBaseImage: "py",
			BaseImageAliases: map[string]string{"py": "python:3.12-slim"},
		}
		got, source, aliased, err := chooseBaseImage("", nil, cfg)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got != "python:3.12-slim" || source != "config" || !aliased {
			t.Fatalf("unexpected: got=%q source=%q aliased=%v", got, source, aliased)
		}
	})

	t.Run("error when both flag and arg provided", func(t *testing.T) {
		cfg := AppConfig{DefaultBaseImage: "python:3.12-slim"}
		_, _, _, err := chooseBaseImage("x", []string{"y"}, cfg)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})

	t.Run("error when no default and no input", func(t *testing.T) {
		cfg := AppConfig{DefaultBaseImage: ""}
		_, _, _, err := chooseBaseImage("", nil, cfg)
		if err == nil {
			t.Fatalf("expected error, got nil")
		}
	})
}
