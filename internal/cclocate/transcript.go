package cclocate

import (
	"bufio"
	"encoding/json"
	"os"
	"path/filepath"
)

// transcriptMessage represents a single line in the transcript JSONL.
type transcriptMessage struct {
	Type    string `json:"type"`
	Message struct {
		Role    string            `json:"role"`
		Content []json.RawMessage `json:"content"`
	} `json:"message"`
}

// contentBlock represents a content block in a transcript message.
type contentBlock struct {
	Type  string          `json:"type"`
	Name  string          `json:"name"`
	Input json.RawMessage `json:"input"`
}

// writeInput represents the input to a Write tool_use.
type writeInput struct {
	FilePath string `json:"file_path"`
}

// findPlanFilesInTranscript reads a transcript JSONL file and finds plan file paths.
// It scans for assistant messages containing Write tool_use calls where the
// file_path is under the given plansDir.
// If all is true, returns all found plan files. Otherwise returns only the latest.
func findPlanFilesInTranscript(transcriptPath, plansDir string, all bool) ([]string, error) {
	f, err := os.Open(transcriptPath)
	if err != nil {
		return nil, err
	}
	defer f.Close()

	// Read all lines first, then scan backwards
	var lines []string
	scanner := bufio.NewScanner(f)
	scanner.Buffer(make([]byte, 0, 1024*1024), 10*1024*1024)
	for scanner.Scan() {
		lines = append(lines, scanner.Text())
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}

	var found []string
	seen := make(map[string]bool)

	// Scan backwards to find latest first
	for i := len(lines) - 1; i >= 0; i-- {
		line := lines[i]
		if line == "" {
			continue
		}

		paths := extractPlanPaths(line, plansDir)
		for _, p := range paths {
			if !seen[p] {
				seen[p] = true
				found = append(found, p)
				if !all {
					return found, nil
				}
			}
		}
	}

	return found, nil
}

// extractPlanPaths extracts plan file paths from a single transcript JSONL line.
func extractPlanPaths(line, plansDir string) []string {
	var msg transcriptMessage
	if err := json.Unmarshal([]byte(line), &msg); err != nil {
		return nil
	}

	if msg.Type != "assistant" && msg.Message.Role != "assistant" {
		return nil
	}

	var paths []string
	for _, raw := range msg.Message.Content {
		var block contentBlock
		if err := json.Unmarshal(raw, &block); err != nil {
			continue
		}
		if block.Type != "tool_use" || block.Name != "Write" {
			continue
		}

		var input writeInput
		if err := json.Unmarshal(block.Input, &input); err != nil {
			continue
		}

		cleanPath := filepath.Clean(input.FilePath)
		if IsUnderDir(cleanPath, plansDir) {
			paths = append(paths, cleanPath)
		}
	}

	return paths
}
