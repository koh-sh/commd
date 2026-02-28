package pane

// AutoDetect returns the best available PaneSpawner for the current environment.
func AutoDetect() PaneSpawner {
	spawners := []PaneSpawner{
		&WezTermSpawner{},
	}
	for _, s := range spawners {
		if s.Available() {
			return s
		}
	}
	return &DirectSpawner{}
}

// ByName returns a PaneSpawner by name, falling back to AutoDetect if not found.
func ByName(name string) PaneSpawner {
	switch name {
	case NameWezTerm:
		return &WezTermSpawner{}
	case NameTmux:
		// tmux support is not yet implemented; fall back to DirectSpawner.
		return &DirectSpawner{}
	case NameAuto, "":
		return AutoDetect()
	default:
		return AutoDetect()
	}
}
