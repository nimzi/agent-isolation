package aishell

import (
	"os"
	"os/exec"
	"runtime"
	"strings"
	"syscall"
)

func isTTY() bool {
	in, _ := os.Stdin.Stat()
	out, _ := os.Stdout.Stat()
	return (in.Mode()&os.ModeCharDevice) != 0 && (out.Mode()&os.ModeCharDevice) != 0
}

func execReplace(bin string, args []string) error {
	// On Linux/macOS we can replace the current process for better TTY/signal behavior.
	if runtime.GOOS != "windows" {
		path, err := exec.LookPath(bin)
		if err != nil {
			return err
		}
		return syscall.Exec(path, append([]string{bin}, args...), os.Environ())
	}
	// Fallback for Windows.
	c := exec.Command(bin, args...)
	c.Stdin = os.Stdin
	c.Stdout = os.Stdout
	c.Stderr = os.Stderr
	return c.Run()
}

func redactSecrets(out string) string {
	lines := strings.Split(out, "\n")
	for i, ln := range lines {
		if strings.Contains(ln, "TOKEN=") || strings.Contains(ln, "KEY=") {
			parts := strings.SplitN(ln, "=", 2)
			if len(parts) == 2 {
				lines[i] = parts[0] + "=***"
			}
		}
	}
	return strings.Join(lines, "\n")
}
