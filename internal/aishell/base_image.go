package aishell

import (
	"fmt"
	"strings"
)

// resolveBaseImage resolves an input that may be an alias key.
// If input matches a user-defined alias in cfg, the mapped image ref is returned.
// Otherwise the input is treated as a literal image reference.
func resolveBaseImage(input string, cfg AppConfig) (resolved string, usedAlias bool, err error) {
	input = strings.TrimSpace(input)
	if err := validateNonEmptyImageRef(input); err != nil {
		return "", false, err
	}
	if cfg.BaseImageAliases != nil {
		if v, ok := cfg.BaseImageAliases[input]; ok {
			v = strings.TrimSpace(v)
			if err := validateNonEmptyImageRef(v); err != nil {
				return "", false, fmt.Errorf("alias %q maps to invalid image reference: %w", input, err)
			}
			return v, true, nil
		}
	}
	return input, false, nil
}

// chooseBaseImage selects the base image (docker/Dockerfile FROM) from (flag,args,config).
//
// Precedence:
// - explicit flag (--base-image)
// - positional arg (up [BASE_IMAGE_OR_ALIAS])
// - config defaultBaseImage
//
// It resolves aliases using cfg.BaseImageAliases.
func chooseBaseImage(flagVal string, args []string, cfg AppConfig) (base string, source string, usedAlias bool, err error) {
	flagVal = strings.TrimSpace(flagVal)
	if flagVal != "" && len(args) > 0 {
		return "", "", false, fmt.Errorf("base image specified twice (positional arg and --base-image)")
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
		return "", "", false, fmt.Errorf("no base image specified; set a default with: ai-shell config set-default-base-image <image>")
	}

	res, aliased, err := resolveBaseImage(raw, cfg)
	if err != nil {
		return "", "", false, err
	}
	return res, source, aliased, nil
}
