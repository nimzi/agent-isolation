package aishell

import "testing"

func TestChooseBaseImage(t *testing.T) {
	t.Run("flag wins and resolves alias", func(t *testing.T) {
		cfg := AppConfig{
			DefaultBaseImage: "py",
			BaseImageAliases: map[string]AliasEntry{
				"u24": {Image: "ubuntu:24.04", Family: "apt"},
				"py":  {Image: "python:3.12-slim", Family: "apt"},
			},
		}
		got, family, source, err := chooseBaseImage("u24", nil, cfg)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got != "ubuntu:24.04" || source != "flag" || family != "apt" {
			t.Fatalf("unexpected: got=%q family=%q source=%q", got, family, source)
		}
	})

	t.Run("arg used when flag empty", func(t *testing.T) {
		cfg := AppConfig{
			DefaultBaseImage: "py",
			BaseImageAliases: map[string]AliasEntry{
				"deb": {Image: "debian:12-slim", Family: "apt"},
				"py":  {Image: "python:3.12-slim", Family: "apt"},
			},
		}
		got, family, source, err := chooseBaseImage("", []string{"deb"}, cfg)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got != "debian:12-slim" || source != "arg" || family != "apt" {
			t.Fatalf("unexpected: got=%q family=%q source=%q", got, family, source)
		}
	})

	t.Run("default resolves via alias", func(t *testing.T) {
		cfg := AppConfig{
			DefaultBaseImage: "py",
			BaseImageAliases: map[string]AliasEntry{
				"py": {Image: "python:3.12-slim", Family: "apt"},
			},
		}
		got, family, source, err := chooseBaseImage("", nil, cfg)
		if err != nil {
			t.Fatalf("expected nil error, got %v", err)
		}
		if got != "python:3.12-slim" || source != "config" || family != "apt" {
			t.Fatalf("unexpected: got=%q family=%q source=%q", got, family, source)
		}
	})

	t.Run("error when both flag and arg provided", func(t *testing.T) {
		cfg := AppConfig{
			DefaultBaseImage: "py",
			BaseImageAliases: map[string]AliasEntry{
				"py": {Image: "python:3.12-slim", Family: "apt"},
			},
		}
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

	t.Run("error when non-alias literal is used", func(t *testing.T) {
		cfg := AppConfig{
			DefaultBaseImage: "ubu",
			BaseImageAliases: map[string]AliasEntry{
				"ubu": {Image: "ubuntu:24.04", Family: "apt"},
			},
		}
		_, _, _, err := chooseBaseImage("ubuntu:24.04", nil, cfg)
		if err == nil {
			t.Fatalf("expected error for non-alias input, got nil")
		}
	})
}
