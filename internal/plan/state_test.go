package plan

import (
	"os"
	"path/filepath"
	"testing"
)

func TestStatePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"plan.md", "plan.md.reviewed.json"},
		{"/home/user/plans/my-plan.md", "/home/user/plans/my-plan.md.reviewed.json"},
	}
	for _, tt := range tests {
		got := StatePath(tt.input)
		if got != tt.want {
			t.Errorf("StatePath(%q) = %q, want %q", tt.input, got, tt.want)
		}
	}
}

func TestContentHash(t *testing.T) {
	s1 := &Step{Title: "Step 1", Body: "body text"}
	s2 := &Step{Title: "Step 1", Body: "body text"}
	s3 := &Step{Title: "Step 1", Body: "different body"}

	// Same content produces same hash
	if contentHash(s1) != contentHash(s2) {
		t.Error("identical steps should have same hash")
	}

	// Different content produces different hash
	if contentHash(s1) == contentHash(s3) {
		t.Error("different steps should have different hash")
	}

	// Hash is 16 hex chars (8 bytes)
	hash := contentHash(s1)
	if len(hash) != 16 {
		t.Errorf("hash length = %d, want 16", len(hash))
	}
}

func TestLoadViewedStateFileNotFound(t *testing.T) {
	state := LoadViewedState("/nonexistent/path.json")
	if state == nil {
		t.Fatal("should return non-nil state")
		return
	}
	if len(state.Steps) != 0 {
		t.Error("should return empty steps map")
	}
}

func TestLoadViewedStateInvalidJSON(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "bad.json")
	if err := os.WriteFile(tmp, []byte("not json"), 0o644); err != nil {
		t.Fatal(err)
	}

	state := LoadViewedState(tmp)
	if state == nil {
		t.Fatal("should return non-nil state")
		return
	}
	if len(state.Steps) != 0 {
		t.Error("should return empty steps map for invalid JSON")
	}
}

func TestSaveAndLoadRoundTrip(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "state.json")

	original := NewViewedState()
	original.Steps["Step 1"] = "abc123def456abcd"
	original.Steps["Step 2"] = "1234567890abcdef"

	if err := SaveViewedState(tmp, original); err != nil {
		t.Fatalf("SaveViewedState: %v", err)
	}

	loaded := LoadViewedState(tmp)
	if len(loaded.Steps) != 2 {
		t.Fatalf("loaded steps count = %d, want 2", len(loaded.Steps))
	}
	if loaded.Steps["Step 1"] != original.Steps["Step 1"] {
		t.Error("Step 1 hash mismatch")
	}
	if loaded.Steps["Step 2"] != original.Steps["Step 2"] {
		t.Error("Step 2 hash mismatch")
	}
}

func TestIsStepViewed(t *testing.T) {
	s := &Step{Title: "Step 1", Body: "body"}
	state := NewViewedState()

	// Not tracked yet
	if state.IsStepViewed(s) {
		t.Error("should not be viewed initially")
	}

	// Mark viewed
	state.MarkViewed(s)
	if !state.IsStepViewed(s) {
		t.Error("should be viewed after MarkViewed")
	}

	// Change body (stale hash)
	s.Body = "changed body"
	if state.IsStepViewed(s) {
		t.Error("should not be viewed after content change")
	}
}

func TestMarkAndUnmarkViewed(t *testing.T) {
	s := &Step{Title: "Step 1", Body: "body"}
	state := NewViewedState()

	state.MarkViewed(s)
	if _, ok := state.Steps["Step 1"]; !ok {
		t.Error("MarkViewed should add entry")
	}

	state.UnmarkViewed(s)
	if _, ok := state.Steps["Step 1"]; ok {
		t.Error("UnmarkViewed should remove entry")
	}
}

func TestLoadViewedStateNullSteps(t *testing.T) {
	tmp := filepath.Join(t.TempDir(), "null.json")
	if err := os.WriteFile(tmp, []byte(`{}`), 0o644); err != nil {
		t.Fatal(err)
	}

	state := LoadViewedState(tmp)
	if state.Steps == nil {
		t.Error("Steps map should be initialized even for empty JSON object")
	}
}

func TestSaveViewedStateError(t *testing.T) {
	err := SaveViewedState("/nonexistent/dir/state.json", NewViewedState())
	if err == nil {
		t.Error("should return error for invalid path")
	}
}
