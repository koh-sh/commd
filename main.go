package main

import (
	"github.com/alecthomas/kong"
	"github.com/koh-sh/commd/cmd"
)

var version = "dev"

func main() {
	var cli cmd.CLI
	ctx := kong.Parse(&cli,
		kong.Name("commd"),
		kong.Description("Interactive Markdown reviewer"),
		kong.UsageOnError(),
		kong.Vars{"version": version},
	)
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}
