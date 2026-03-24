package aishell

import (
	"fmt"
	"strings"
)

// resolveBaseImage resolves an alias key to its image ref and family.
// Non-alias inputs are rejected — every base image must be defined as an alias.
func resolveBaseImage(input string, cfg AppConfig) (image string, family string, err error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", "", fmt.Errorf("base image must not be empty")
	}
	if cfg.BaseImageAliases != nil {
		if entry, ok := cfg.BaseImageAliases[input]; ok {
			img := strings.TrimSpace(entry.Image)
			if err := validateNonEmptyImageRef(img); err != nil {
				return "", "", fmt.Errorf("alias %q maps to invalid image reference: %w", input, err)
			}
			return img, entry.Family, nil
		}
	}
	return "", "", fmt.Errorf("unknown alias %q: define it first with: ai-shell config alias set %s <image> <family>", input, input)
}

// chooseBaseImage selects the base image (docker/Dockerfile FROM) from (flag,args,config).
//
// Precedence:
// - explicit flag (--base-image)
// - positional arg (up [ALIAS])
// - config defaultBaseImage
//
// All inputs must be alias names defined in cfg.BaseImageAliases.
func chooseBaseImage(flagVal string, args []string, cfg AppConfig) (base string, family string, source string, err error) {
	flagVal = strings.TrimSpace(flagVal)
	if flagVal != "" && len(args) > 0 {
		return "", "", "", fmt.Errorf("base image specified twice (positional arg and --base-image)")
	}

	raw := ""
	source = ""
	switch {
	case flagVal != "":
		raw = flagVal
		source = "flag"
	case len(args) == 1:
		raw = strings.TrimSpace(args[0])
		source = "arg"
	default:
		raw = strings.TrimSpace(cfg.DefaultBaseImage)
		source = "config"
	}

	if raw == "" {
		return "", "", "", fmt.Errorf("no base image specified; set a default with: ai-shell config set-default-base-image <alias>")
	}

	img, fam, err := resolveBaseImage(raw, cfg)
	if err != nil {
		return "", "", "", err
	}
	return img, fam, source, nil
}
