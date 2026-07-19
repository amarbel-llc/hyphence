package main

import (
	"code.linenisgreat.com/hyphence/go/cli_main"
	"code.linenisgreat.com/hyphence/go/commands_hyphence"
)

// version is stamped at link time via `-X main.version` (igloo's
// buildGoApplication injects it from the flake's version arg, which reads
// version.env's HYPHENCE_VERSION). It is a package-level var so the linker has a
// target; the four format-only subcommands don't surface it today.
var version = "dev"

func main() {
	cli_main.Run(commands_hyphence.GetUtility(), "hyphence")
}
