package cclocate

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
)

// Options configures the plan file location process.
type Options struct {
	TranscriptPath string
	CWD            string
	All            bool
}

// HookInput represents the JSON input from a Claude Code hook.
type HookInput struct {
	SessionID      string `json:"session_id"`
	TranscriptPath string `json:"transcript_path"`
	CWD            string `json:"cwd"`
}

// ParseHookInput reads and parses hook JSON input from a reader.
func ParseHookInput(r io.Reader) (*HookInput, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading stdin: %w", err)
	}

	var input HookInput
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parsing hook input: %w", err)
	}

	return &input, nil
}

// LocatePlanFile finds plan file paths from a transcript and settings.
// Returns a list of validated (existing) plan file paths.
func LocatePlanFile(opts Options) ([]string, error) {
	if opts.CWD == "" {
		opts.CWD = "."
	}

	if opts.TranscriptPath == "" {
		return nil, fmt.Errorf("transcript path is required")
	}

	plansDir := ResolvePlansDir(opts.CWD)
	if plansDir == "" {
		return nil, fmt.Errorf("could not resolve plans directory")
	}

	paths, err := findPlanFilesInTranscript(opts.TranscriptPath, plansDir, opts.All)
	if err != nil {
		return nil, fmt.Errorf("scanning transcript: %w", err)
	}

	// Validate that files actually exist
	var validated []string
	for _, p := range paths {
		if _, err := os.Stat(p); err == nil {
			validated = append(validated, p)
		}
	}

	return validated, nil
}

// IsUnderDir checks if filePath is under dir.
func IsUnderDir(filePath, dir string) bool {
	if dir == "" {
		return false
	}
	cleanPath := filepath.Clean(filePath)
	cleanDir := filepath.Clean(dir) + string(filepath.Separator)
	return strings.HasPrefix(cleanPath, cleanDir)
}
