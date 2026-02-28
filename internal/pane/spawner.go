package pane

import "context"

// Spawner name constants.
const (
	NameDirect  = "direct"
	NameWezTerm = "wezterm"
	NameTmux    = "tmux"
	NameAuto    = "auto"
)

// PaneSpawner abstracts terminal multiplexer pane operations.
type PaneSpawner interface {
	// SpawnAndWait spawns a command in a new pane and waits for completion.
	SpawnAndWait(ctx context.Context, cmd string, args []string) error
	// Available returns whether this spawner can be used in the current environment.
	Available() bool
	// Name returns the multiplexer name.
	Name() string
}
