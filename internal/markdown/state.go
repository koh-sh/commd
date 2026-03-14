package markdown

import (
	"crypto/sha256"
	"encoding/json"
	"fmt"
	"os"
)

// ViewedState tracks which sections have been viewed and their content hashes.
type ViewedState struct {
	Sections map[string]string `json:"sections"` // title -> content hash
}

// NewViewedState creates an empty ViewedState.
func NewViewedState() *ViewedState {
	return &ViewedState{Sections: make(map[string]string)}
}

// StatePath returns the sidecar file path for persisting viewed state.
func StatePath(filePath string) string {
	return filePath + ".reviewed.json"
}

// contentHash computes a truncated SHA-256 hash of a section's title and body.
func contentHash(s *Section) string {
	h := sha256.Sum256([]byte(s.Title + "\x00" + s.Body))
	return fmt.Sprintf("%x", h[:8])
}

// LoadViewedState reads a viewed state file. Returns an empty state on any error.
func LoadViewedState(path string) *ViewedState {
	data, err := os.ReadFile(path)
	if err != nil {
		return NewViewedState()
	}
	var state ViewedState
	if err := json.Unmarshal(data, &state); err != nil {
		return NewViewedState()
	}
	if state.Sections == nil {
		state.Sections = make(map[string]string)
	}
	return &state
}

// SaveViewedState writes the viewed state to a JSON file.
func SaveViewedState(path string, state *ViewedState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("marshaling viewed state: %w", err)
	}
	if err := os.WriteFile(path, append(data, '\n'), 0o644); err != nil {
		return fmt.Errorf("writing viewed state: %w", err)
	}
	return nil
}

// IsSectionViewed returns true if the section's title is tracked and its content hash matches.
func (vs *ViewedState) IsSectionViewed(s *Section) bool {
	hash, ok := vs.Sections[s.Title]
	if !ok {
		return false
	}
	return hash == contentHash(s)
}

// MarkViewed records a section as viewed with its current content hash.
func (vs *ViewedState) MarkViewed(s *Section) {
	vs.Sections[s.Title] = contentHash(s)
}

// UnmarkViewed removes a section's viewed status.
func (vs *ViewedState) UnmarkViewed(s *Section) {
	delete(vs.Sections, s.Title)
}
