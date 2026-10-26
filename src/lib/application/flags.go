package application

import "dockpipe/src/lib/application/internal/clioptions"

// CliOpts holds parsed CLI flags before the command separator.
type CliOpts = clioptions.CliOpts

// ParseFlags parses argv until the command separator or end of input.
func ParseFlags(repoRoot string, argv []string) ([]string, *CliOpts, error) {
	return clioptions.ParseFlags(repoRoot, argv)
}
