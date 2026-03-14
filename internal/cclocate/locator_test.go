package cclocate

import (
	"fmt"
	"io"
	"path/filepath"
	"strings"
	"testing"
)

// errReader is a reader that always returns an error.
type errReader struct{}

func (e errReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

var _ io.Reader = errReader{}

func TestParseHookInputReadError(t *testing.T) {
	_, err := ParseHookInput(errReader{})
	if err == nil {
		t.Fatal("expected error for failing reader")
	}
	if !strings.Contains(err.Error(), "reading stdin") {
		t.Errorf("error = %q, want to contain 'reading stdin'", err.Error())
	}
}

func TestParseHookInput(t *testing.T) {
	t.Run("valid JSON", func(t *testing.T) {
		r := strings.NewReader(`{"session_id":"abc","transcript_path":"/tmp/t.jsonl","cwd":"/home"}`)
		input, err := ParseHookInput(r)
		if err != nil {
			t.Fatal(err)
		}
		if input.SessionID != "abc" {
			t.Errorf("session_id = %s, want abc", input.SessionID)
		}
		if input.TranscriptPath != "/tmp/t.jsonl" {
			t.Errorf("transcript_path = %s, want /tmp/t.jsonl", input.TranscriptPath)
		}
		if input.CWD != "/home" {
			t.Errorf("cwd = %s, want /home", input.CWD)
		}
	})

	t.Run("invalid JSON", func(t *testing.T) {
		r := strings.NewReader(`not json`)
		_, err := ParseHookInput(r)
		if err == nil {
			t.Fatal("expected error for invalid JSON")
		}
		if !strings.Contains(err.Error(), "parsing hook input") {
			t.Errorf("error = %q, want to contain 'parsing hook input'", err.Error())
		}
	})

	t.Run("empty input", func(t *testing.T) {
		r := strings.NewReader(``)
		_, err := ParseHookInput(r)
		if err == nil {
			t.Fatal("expected error for empty input")
		}
		if !strings.Contains(err.Error(), "parsing hook input") {
			t.Errorf("error = %q, want to contain 'parsing hook input'", err.Error())
		}
	})
}

func TestLocatePlanFileEmptyTranscript(t *testing.T) {
	_, err := LocatePlanFile(Options{TranscriptPath: ""})
	if err == nil {
		t.Fatal("expected error for empty transcript path")
	}
	if !strings.Contains(err.Error(), "transcript path is required") {
		t.Errorf("error = %q, want to contain 'transcript path is required'", err.Error())
	}
}

func TestIsUnderDir(t *testing.T) {
	tests := []struct {
		name     string
		filePath string
		dir      string
		want     bool
	}{
		{
			name:     "file under dir",
			filePath: "/home/user/.claude/plans/my-plan.md",
			dir:      "/home/user/.claude/plans",
			want:     true,
		},
		{
			name:     "file not under dir",
			filePath: "/home/user/projects/src/main.go",
			dir:      "/home/user/.claude/plans",
			want:     false,
		},
		{
			name:     "dir with trailing separator",
			filePath: "/home/user/.claude/plans/my-plan.md",
			dir:      "/home/user/.claude/plans/",
			want:     true,
		},
		{
			name:     "similar prefix but different dir",
			filePath: "/home/user/.claude/plans-backup/my-plan.md",
			dir:      "/home/user/.claude/plans",
			want:     false,
		},
		{
			name:     "empty dir",
			filePath: "/home/user/.claude/plans/my-plan.md",
			dir:      "",
			want:     false,
		},
		{
			name:     "nested file under dir",
			filePath: filepath.Join("/home/user/.claude/plans", "subdir", "my-plan.md"),
			dir:      "/home/user/.claude/plans",
			want:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := IsUnderDir(tt.filePath, tt.dir)
			if got != tt.want {
				t.Errorf("IsUnderDir(%q, %q) = %v, want %v", tt.filePath, tt.dir, got, tt.want)
			}
		})
	}
}

func TestLocatePlanFileNoPlansDir(t *testing.T) {
	// Use a CWD with no settings files and a fake home to avoid resolving default
	tmpDir := t.TempDir()
	_, err := LocatePlanFile(Options{
		TranscriptPath: "/nonexistent/transcript.jsonl",
		CWD:            tmpDir,
	})
	if err == nil {
		t.Fatal("expected error when transcript doesn't exist")
	}
	if !strings.Contains(err.Error(), "scanning transcript") {
		t.Errorf("error = %q, want to contain 'scanning transcript'", err.Error())
	}
}
