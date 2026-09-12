package cmd

import (
	"fmt"

	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/kong"
)

// CLI is the top-level command structure for commd.
type CLI struct {
	Review   ReviewCmd  `cmd:"" default:"withargs" help:"Review a Markdown file in TUI (default command: commd file.md)"`
	PR       PRCmd      `cmd:"" help:"Review Markdown files in a GitHub PR"`
	Cclocate LocateCmd  `cmd:"cclocate" help:"Locate file path from Claude Code transcript"`
	Cchook   HookCmd    `cmd:"cchook" help:"Run as Claude Code hook (PreToolUse ExitPlanMode or PostToolUse Write|Edit)"`
	Version  VersionCmd `cmd:"" help:"Show version"`
}

// HookCmd is the hook subcommand.
type HookCmd struct {
	Spawner string `enum:"wezterm,tmux,auto" default:"auto" help:"Force specific multiplexer (wezterm|tmux|auto)"`
	Theme   string `enum:"dark,light" default:"dark" help:"Color theme (dark|light)"`
}

// ReviewCmd is the review subcommand.
type ReviewCmd struct {
	Files       []string `arg:"" optional:"" name:"file" help:"Path to the Markdown file (with --diff: zero or more changed files)"`
	Output      string   `enum:"clipboard,stdout,file" default:"clipboard" help:"Output method (clipboard|stdout|file)"`
	OutputPath  string   `help:"File path for file output" type:"path"`
	Theme       string   `enum:"dark,light" default:"dark" help:"Color theme (dark|light)"`
	TrackViewed bool     `help:"Persist viewed state to sidecar file for change detection across sessions"`
	Diff        bool     `help:"Review local git changes in diff view; with no file, pick from changed .md files"`
	Base        string   `help:"Git ref to diff against (default: HEAD; requires --diff)"`

	teaOpts []tea.ProgramOption // for testing: override tea.NewProgram options
}

// PRCmd is the pr subcommand for reviewing Markdown files in a GitHub PR.
type PRCmd struct {
	URL   string `arg:"" help:"GitHub PR URL (e.g. https://github.com/owner/repo/pull/123)"`
	File  string `help:"Review a specific file instead of showing file picker"`
	Theme string `enum:"dark,light" default:"dark" help:"Color theme (dark|light)"`

	teaOpts []tea.ProgramOption // for testing: override tea.NewProgram options
}

// LocateCmd is the locate subcommand.
type LocateCmd struct {
	Transcript string `help:"Path to transcript JSONL file" type:"existingfile"`
	CWD        string `help:"Working directory for resolving relative plansDirectory" default:"." type:"existingdir"`
	Stdin      bool   `help:"Read hook JSON input from stdin"`
	All        bool   `help:"Output all plan files found in transcript"`
}

// VersionCmd is the version subcommand. The version string comes from the
// kong.Vars bound in main, not from a flag.
type VersionCmd struct{}

// Run executes the version subcommand.
func (v *VersionCmd) Run(vars kong.Vars) error {
	fmt.Println("commd version " + vars["version"])
	return nil
}
