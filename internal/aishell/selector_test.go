package aishell

import (
	"strings"
	"testing"
)

func TestMatchTarget_InstanceIDPrefix_UniqueResolves(t *testing.T) {
	instances := []ManagedInstance{
		{Workdir: "/a", InstanceID: "a1b2c3d4e5", Container: "ai-agent-shell-a1b2c3d4e5", Status: "running"},
		{Workdir: "/b", InstanceID: "a1b2ffffaa", Container: "ai-agent-shell-a1b2ffffaa", Status: "exited"},
	}

	got, err := matchTarget("a1b2c3", instances)
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}
	if got.InstanceID != "a1b2c3d4e5" {
		t.Fatalf("expected iid %q, got %q", "a1b2c3d4e5", got.InstanceID)
	}
}

func TestMatchTarget_InstanceIDPrefix_AmbiguousErrors(t *testing.T) {
	instances := []ManagedInstance{
		{Workdir: "/a", InstanceID: "a1b2c3d4e5", Container: "ai-agent-shell-a1b2c3d4e5", Status: "running"},
		{Workdir: "/b", InstanceID: "a1b2ffffaa", Container: "ai-agent-shell-a1b2ffffaa", Status: "exited"},
	}

	_, err := matchTarget("a1b2", instances)
	if err == nil {
		t.Fatalf("expected error, got nil")
	}
	if !strings.Contains(err.Error(), "ambiguous target") {
		t.Fatalf("expected ambiguous target error, got: %v", err)
	}
	if !strings.Contains(err.Error(), "a1b2c3d4e5") || !strings.Contains(err.Error(), "a1b2ffffaa") {
		t.Fatalf("expected candidates listed in error, got: %v", err)
	}
}

func TestUniquePrefixLen(t *testing.T) {
	t.Run("unique at min", func(t *testing.T) {
		ids := []string{"abcd111111", "abce222222", "abcf333333"}
		if got := uniquePrefixLen(ids, 4, 10); got != 4 {
			t.Fatalf("expected 4, got %d", got)
		}
	})

	t.Run("collision increases length", func(t *testing.T) {
		ids := []string{"abcd111111", "abcd222222"}
		if got := uniquePrefixLen(ids, 4, 10); got != 5 {
			t.Fatalf("expected 5, got %d", got)
		}
	})

	t.Run("duplicate ids return 0", func(t *testing.T) {
		ids := []string{"aaaaaaaaaa", "aaaaaaaaaa"}
		if got := uniquePrefixLen(ids, 4, 10); got != 0 {
			t.Fatalf("expected 0, got %d", got)
		}
	})
}
