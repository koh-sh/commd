package cclocate

import (
	"encoding/json"
	"os"
	"path/filepath"
)

// defaultPlansDir returns the default plans directory path.
func defaultPlansDir() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ""
	}
	return filepath.Join(home, ".claude", "plans")
}

// ResolvePlansDir resolves the plansDirectory using the settings chain:
//  1. {cwd}/.claude/settings.local.json
//  2. {cwd}/.claude/settings.json
//  3. ~/.claude/settings.json
//  4. default: ~/.claude/plans/
//
// Relative paths are resolved from cwd.
func ResolvePlansDir(cwd string) string {
	settingsFiles := []string{
		filepath.Join(cwd, ".claude", "settings.local.json"),
		filepath.Join(cwd, ".claude", "settings.json"),
	}

	home, err := os.UserHomeDir()
	if err == nil {
		settingsFiles = append(settingsFiles, filepath.Join(home, ".claude", "settings.json"))
	}

	for _, path := range settingsFiles {
		dir := readPlansDirFromSettings(path)
		if dir != "" {
			if !filepath.IsAbs(dir) {
				dir = filepath.Join(cwd, dir)
			}
			return filepath.Clean(dir)
		}
	}

	return defaultPlansDir()
}

// readPlansDirFromSettings reads plansDirectory from a settings JSON file.
// Returns empty string if file doesn't exist, is invalid, or doesn't contain plansDirectory.
func readPlansDirFromSettings(path string) string {
	data, err := os.ReadFile(path)
	if err != nil {
		return ""
	}

	var settings struct {
		PlansDirectory string `json:"plansDirectory"`
	}
	if err := json.Unmarshal(data, &settings); err != nil {
		return ""
	}

	return settings.PlansDirectory
}
