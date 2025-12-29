package main

import (
	"os"

	"github.com/nimzi/agent-isolation/internal/aishell"
)

// ai-shell is the canonical CLI entrypoint.
//
// Note: the CLI supports --home / AI_SHELL_HOME so an installed binary can
// locate the Docker build context (typically installed under
// /usr/local/share/ai-shell/docker) without embedding assets.
func main() {
	os.Exit(aishell.Main())
}
