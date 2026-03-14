package cclocate

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFindPlanFilesInTranscript(t *testing.T) {
	transcriptPath := filepath.Join("testdata", "transcript-with-plan.jsonl")
	plansDir := "/tmp/test-plans"

	paths, err := findPlanFilesInTranscript(transcriptPath, plansDir, false)
	if err != nil {
		t.Fatalf("findPlanFilesInTranscript() error: %v", err)
	}

	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}

	want := filepath.Clean("/tmp/test-plans/jaunty-petting-nebula.md")
	if paths[0] != want {
		t.Errorf("paths[0] = %q, want %q", paths[0], want)
	}
}

func TestFindPlanFilesNoPlan(t *testing.T) {
	transcriptPath := filepath.Join("testdata", "transcript-no-plan.jsonl")
	plansDir := "/tmp/test-plans"

	paths, err := findPlanFilesInTranscript(transcriptPath, plansDir, false)
	if err != nil {
		t.Fatalf("findPlanFilesInTranscript() error: %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("len(paths) = %d, want 0", len(paths))
	}
}

func TestFindPlanFilesMultiplePlans(t *testing.T) {
	transcriptPath := filepath.Join("testdata", "transcript-multiple-plans.jsonl")
	plansDir := "/tmp/test-plans"

	// Latest only
	paths, err := findPlanFilesInTranscript(transcriptPath, plansDir, false)
	if err != nil {
		t.Fatalf("findPlanFilesInTranscript() error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}
	want := filepath.Clean("/tmp/test-plans/plan-b.md")
	if paths[0] != want {
		t.Errorf("latest path = %q, want %q", paths[0], want)
	}

	// All
	allPaths, err := findPlanFilesInTranscript(transcriptPath, plansDir, true)
	if err != nil {
		t.Fatalf("findPlanFilesInTranscript(all) error: %v", err)
	}
	if len(allPaths) != 2 {
		t.Fatalf("len(allPaths) = %d, want 2", len(allPaths))
	}
	// Reverse order since we scan backwards
	if allPaths[0] != filepath.Clean("/tmp/test-plans/plan-b.md") {
		t.Errorf("allPaths[0] = %q, want plan-b.md", allPaths[0])
	}
	if allPaths[1] != filepath.Clean("/tmp/test-plans/plan-a.md") {
		t.Errorf("allPaths[1] = %q, want plan-a.md", allPaths[1])
	}
}

func TestFindPlanFilesMalformed(t *testing.T) {
	transcriptPath := filepath.Join("testdata", "transcript-malformed.jsonl")
	plansDir := "/tmp/test-plans"

	paths, err := findPlanFilesInTranscript(transcriptPath, plansDir, true)
	if err != nil {
		t.Fatalf("findPlanFilesInTranscript() error: %v", err)
	}

	// Should find the valid line and skip malformed ones
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}
}

func TestFindPlanFilesWrongDir(t *testing.T) {
	transcriptPath := filepath.Join("testdata", "transcript-with-plan.jsonl")
	plansDir := "/some/other/dir"

	paths, err := findPlanFilesInTranscript(transcriptPath, plansDir, false)
	if err != nil {
		t.Fatalf("findPlanFilesInTranscript() error: %v", err)
	}

	if len(paths) != 0 {
		t.Errorf("len(paths) = %d, want 0 (wrong plansDir)", len(paths))
	}
}

func TestFindPlanFilesNonExistent(t *testing.T) {
	_, err := findPlanFilesInTranscript("/nonexistent/file.jsonl", "/tmp", false)
	if err == nil {
		t.Fatal("expected error for nonexistent file")
	}
	if !strings.Contains(err.Error(), "no such file") {
		t.Errorf("error = %q, want to contain 'no such file'", err.Error())
	}
}

func TestFindPlanFilesWithEmptyLines(t *testing.T) {
	// Transcript with empty lines between entries
	tmpDir := t.TempDir()
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")

	plansDir := filepath.Join(tmpDir, "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	content := "\n" + // empty line at start
		`{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"` + filepath.Join(plansDir, "plan.md") + `"}}]}}` + "\n" +
		"\n" + // empty line in middle
		"\n" // empty line at end
	if err := os.WriteFile(transcriptFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := findPlanFilesInTranscript(transcriptFile, plansDir, true)
	if err != nil {
		t.Fatalf("findPlanFilesInTranscript() error: %v", err)
	}
	if len(paths) != 1 {
		t.Errorf("len(paths) = %d, want 1", len(paths))
	}
}

func TestFindPlanFilesWithBadContentBlock(t *testing.T) {
	// Content block with invalid JSON in the raw content
	tmpDir := t.TempDir()
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")

	// content array has an invalid JSON element
	content := `{"type":"assistant","message":{"role":"assistant","content":["not a valid block"]}}` + "\n"
	if err := os.WriteFile(transcriptFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := findPlanFilesInTranscript(transcriptFile, filepath.Join(tmpDir, "plans"), true)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for invalid content block, got %d", len(paths))
	}
}

func TestFindPlanFilesWithBadInput(t *testing.T) {
	// Tool use with invalid JSON in the input field
	tmpDir := t.TempDir()
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")

	content := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":"not-an-object"}]}}` + "\n"
	if err := os.WriteFile(transcriptFile, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := findPlanFilesInTranscript(transcriptFile, filepath.Join(tmpDir, "plans"), true)
	if err != nil {
		t.Fatalf("error: %v", err)
	}
	if len(paths) != 0 {
		t.Errorf("expected 0 paths for invalid input field, got %d", len(paths))
	}
}

func TestLocatePlanFile(t *testing.T) {
	// Create a temp plan file so validation passes
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}
	planFile := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planFile, []byte("# Test"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a settings file pointing to plansDir
	settingsDir := filepath.Join(tmpDir, ".claude")
	settingsFile := filepath.Join(settingsDir, "settings.json")
	settingsContent := `{"plansDirectory": "` + plansDir + `"}`
	if err := os.WriteFile(settingsFile, []byte(settingsContent), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a transcript that references the plan file
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	transcriptContent := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","id":"toolu_001","name":"Write","input":{"file_path":"` + planFile + `","content":"# Test"}}]},"sessionId":"test","timestamp":"2025-01-01T00:00:00Z"}` + "\n"
	if err := os.WriteFile(transcriptFile, []byte(transcriptContent), 0o644); err != nil {
		t.Fatal(err)
	}

	paths, err := LocatePlanFile(Options{
		TranscriptPath: transcriptFile,
		CWD:            tmpDir,
	})
	if err != nil {
		t.Fatalf("LocatePlanFile() error: %v", err)
	}
	if len(paths) != 1 {
		t.Fatalf("len(paths) = %d, want 1", len(paths))
	}
	if paths[0] != filepath.Clean(planFile) {
		t.Errorf("paths[0] = %q, want %q", paths[0], filepath.Clean(planFile))
	}
}
