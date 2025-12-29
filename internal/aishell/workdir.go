package aishell

import (
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

func CanonicalWorkdir(p string) (string, error) {
	if p == "" {
		var err error
		p, err = os.Getwd()
		if err != nil {
			return "", err
		}
	}
	p = expandUser(p)
	abs, err := filepath.Abs(p)
	if err != nil {
		return "", err
	}
	real, err := filepath.EvalSymlinks(abs)
	if err != nil {
		return "", err
	}
	st, err := os.Stat(real)
	if err != nil {
		return "", err
	}
	if !st.IsDir() {
		return "", fmt.Errorf("workdir is not a directory: %s", real)
	}
	return real, nil
}

func InstanceID(workdir string) string {
	sum := sha256.Sum256([]byte(workdir))
	return hex.EncodeToString(sum[:])[:10]
}

func NamesFor(workdir, containerBase, volumeBase string) (container string, volume string) {
	iid := InstanceID(workdir)
	return containerBase + "-" + iid, volumeBase + "_" + iid
}

func expandUser(p string) string {
	if p == "" {
		return p
	}
	if p == "~" || strings.HasPrefix(p, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return p
		}
		if p == "~" {
			return home
		}
		return filepath.Join(home, p[2:])
	}
	return p
}
