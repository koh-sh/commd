package main

import (
	"errors"
	"os"

	"github.com/alecthomas/kong"
	"github.com/koh-sh/commd/cmd"
	ghclient "github.com/koh-sh/commd/internal/github"
)

var version = "dev"

func main() {
	var cli cmd.CLI
	ctx := kong.Parse(&cli,
		kong.Name("commd"),
		kong.Description("Interactive Markdown reviewer"),
		kong.UsageOnError(),
		kong.Vars{"version": version},
		// Lazy: only invoked when a Run() method requests *ghclient.Client.
		kong.BindToProvider(ghclient.NewClient),
	)
	err := ctx.Run()
	// The hook signals feedback to Claude Code through its exit code.
	var exitErr cmd.ExitCodeError
	if errors.As(err, &exitErr) {
		os.Exit(exitErr.Code)
	}
	ctx.FatalIfErrorf(err)
}
