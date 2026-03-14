package cmd

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/alecthomas/kong"
	tea "github.com/charmbracelet/bubbletea"
)

func TestVersionCmdRun(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	v := &VersionCmd{}
	err := v.Run(kong.Vars{"version": "1.2.3"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if output != "commd version 1.2.3\n" {
		t.Errorf("output = %q, want %q", output, "commd version 1.2.3\n")
	}
}

func TestLocateCmdRunNoArgs(t *testing.T) {
	l := &LocateCmd{}
	err := l.Run()
	if err == nil {
		t.Fatal("expected error when no transcript or stdin specified")
	}
	if !strings.Contains(err.Error(), "--transcript or --stdin is required") {
		t.Errorf("error = %q, want to contain '--transcript or --stdin is required'", err.Error())
	}
}

func TestLocateCmdRunWithTranscript(t *testing.T) {
	// Create a valid transcript JSONL with a plan file reference
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create plan file
	planFile := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planFile, []byte("# Plan"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create settings
	settingsDir := filepath.Join(tmpDir, ".claude")
	settingsJSON := `{"plansDirectory":"` + plansDir + `"}`
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.local.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create transcript in the correct format:
	// {"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"..."}}]}}
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	transcriptLine := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"` + planFile + `"}}]}}`
	if err := os.WriteFile(transcriptFile, []byte(transcriptLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	l := &LocateCmd{
		Transcript: transcriptFile,
		CWD:        tmpDir,
	}
	err := l.Run()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if output == "" {
		t.Error("expected plan file path in output")
	}
}

func TestLocateCmdRunNoPlanFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty transcript
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptFile, []byte(`{"type":"other"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := &LocateCmd{
		Transcript: transcriptFile,
		CWD:        tmpDir,
	}
	err := l.Run()
	if err == nil {
		t.Fatal("expected error when no plan found")
	}
	if !strings.Contains(err.Error(), "no plan") {
		t.Errorf("error = %q, want to contain 'no plan'", err.Error())
	}
}

func TestWriteReviewOutput(t *testing.T) {
	t.Run("stdout", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := writeReviewOutput("hello", "stdout", "")

		w.Close()
		os.Stdout = old

		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(r); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "hello" {
			t.Errorf("output = %q, want %q", buf.String(), "hello")
		}
	})

	t.Run("file write success", func(t *testing.T) {
		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "output.txt")
		// Create the file first so WriteFile can succeed
		if err := os.WriteFile(outFile, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}

		err := writeReviewOutput("review content", "file", outFile)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "review content" {
			t.Errorf("file content = %q, want %q", string(got), "review content")
		}
	})

	t.Run("file missing output path", func(t *testing.T) {
		err := writeReviewOutput("review", "file", "")
		if err == nil {
			t.Fatal("expected error for empty output path")
		}
		if !strings.Contains(err.Error(), "--output-path is required") {
			t.Errorf("error = %q, want to contain '--output-path is required'", err.Error())
		}
	})

	t.Run("file deleted falls back", func(t *testing.T) {
		// Simulate hook timeout scenario: file was created then deleted.
		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "output.txt")
		// File does not exist at this path (never created), simulating deletion.

		oldErr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		err := writeReviewOutput("fallback content", "file", outFile)

		w.Close()
		os.Stderr = oldErr

		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(r); err != nil {
			t.Fatal(err)
		}
		stderr := buf.String()
		if !strings.Contains(stderr, "was deleted") {
			t.Errorf("stderr should mention file was deleted, got %q", stderr)
		}
		// File should NOT be created by the fallback path.
		if _, err := os.Stat(outFile); err == nil {
			t.Error("output file should not exist after fallback")
		}
	})

	t.Run("file write error returns error", func(t *testing.T) {
		// Create a read-only directory to cause a permission error
		tmpDir := t.TempDir()
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		if err := os.MkdirAll(readOnlyDir, 0o755); err != nil {
			t.Fatal(err)
		}
		outFile := filepath.Join(readOnlyDir, "output.txt")
		// Create the file, then make directory read-only
		if err := os.WriteFile(outFile, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(outFile, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(outFile, 0o644) })

		err := writeReviewOutput("review", "file", outFile)
		if err == nil {
			t.Fatal("expected error for permission denied")
		}
		if !strings.Contains(err.Error(), "writing output file") {
			t.Errorf("error = %q, want to contain 'writing output file'", err.Error())
		}
	})
}

func TestReviewCmdRunFileNotFound(t *testing.T) {
	r := &ReviewCmd{
		File: "/nonexistent/path/plan.md",
	}
	err := r.Run()
	if err == nil {
		t.Fatal("expected error for nonexistent plan file")
	}
	if !strings.Contains(err.Error(), "reading file") {
		t.Errorf("error = %q, want to contain 'reading file'", err.Error())
	}
}

func TestLocateCmdRunStdinParseError(t *testing.T) {
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = w.WriteString("not valid json")
	w.Close()
	os.Stdin = r

	l := &LocateCmd{Stdin: true}
	err := l.Run()
	os.Stdin = oldStdin

	if err == nil {
		t.Fatal("expected error for invalid stdin JSON")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error = %q, want to contain 'parsing'", err.Error())
	}
}

func TestLocateCmdRunLocateError(t *testing.T) {
	tmpDir := t.TempDir()

	l := &LocateCmd{
		Transcript: "/nonexistent/transcript.jsonl",
		CWD:        tmpDir,
	}
	err := l.Run()
	if err == nil {
		t.Fatal("expected error for nonexistent transcript")
	}
	if !strings.Contains(err.Error(), "locating plan file") {
		t.Errorf("error = %q, want to contain 'locating plan file'", err.Error())
	}
}

func TestReviewCmdRunNoTerminal(t *testing.T) {
	tmpDir := t.TempDir()
	planFile := filepath.Join(tmpDir, "plan.md")
	if err := os.WriteFile(planFile, []byte("# Plan\n\n## Step 1\n\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use a pipe with Ctrl+C as input to prevent bubbletea from falling back
	// to /dev/tty, which would start a real TUI and hang the test.
	pr, pw, _ := os.Pipe()
	_, _ = pw.Write([]byte{3}) // Ctrl+C to quit immediately
	pw.Close()

	r := &ReviewCmd{
		File:    planFile,
		Output:  "stdout",
		teaOpts: []tea.ProgramOption{tea.WithInput(pr)},
	}
	err := r.Run()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHookCmdRunExit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envSkip  bool
		wantCode int
	}{
		{
			name:     "parse error",
			input:    "not valid json",
			wantCode: 0,
		},
		{
			name:     "non-plan mode",
			input:    `{"session_id":"test","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","hook_event_name":"PostToolUse","permission_mode":"default","tool_name":"Write","tool_input":{"file_path":"/tmp/file.go"}}`,
			wantCode: 0,
		},
		{
			name:     "skip env",
			input:    `{"session_id":"test","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","hook_event_name":"PostToolUse","permission_mode":"plan","tool_name":"Write","tool_input":{"file_path":"/tmp/file.go"}}`,
			envSkip:  true,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSkip {
				t.Setenv("CC_PLAN_REVIEW_SKIP", "1")
			}
			h := &HookCmd{Spawner: "auto", Theme: "dark"}
			code := h.runExit(strings.NewReader(tt.input))
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestLocateCmdRunStdinMode(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	planFile := filepath.Join(plansDir, "plan.md")
	if err := os.WriteFile(planFile, []byte("# Plan"), 0o644); err != nil {
		t.Fatal(err)
	}

	settingsDir := filepath.Join(tmpDir, ".claude")
	settingsJSON := `{"plansDirectory":"` + plansDir + `"}`
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.local.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	transcriptLine := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"` + planFile + `"}}]}}`
	if err := os.WriteFile(transcriptFile, []byte(transcriptLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create stdin with hook input JSON
	hookInput := `{"session_id":"test","transcript_path":"` + transcriptFile + `","cwd":"` + tmpDir + `"}`
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(hookInput)
	w.Close()
	os.Stdin = r

	// Capture stdout
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	l := &LocateCmd{
		Stdin: true,
	}
	err := l.Run()

	wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rOut); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if output == "" {
		t.Error("expected plan file path in output")
	}
}
