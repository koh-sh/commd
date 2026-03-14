package cchook

import (
	"fmt"
	"io"
	"strings"
	"testing"

	"github.com/koh-sh/commd/internal/cclocate"
)

// errReader is a reader that always returns an error.
type errReader struct{}

func (e errReader) Read([]byte) (int, error) {
	return 0, fmt.Errorf("read error")
}

func TestParseInputReadError(t *testing.T) {
	_, err := ParseInput(errReader{})
	if err == nil {
		t.Fatal("expected error for failing reader")
	}
	if !strings.Contains(err.Error(), "reading") {
		t.Errorf("error = %q, want to contain 'reading'", err.Error())
	}
}

var _ io.Reader = errReader{}

func TestParseInput(t *testing.T) {
	tests := []struct {
		name    string
		json    string
		want    *Input
		wantErr bool
	}{
		{
			name: "valid PostToolUse input",
			json: `{
				"session_id": "eb5b0174-0555-4601-804e-672d68069c89",
				"transcript_path": "/home/user/.claude/projects/test/session.jsonl",
				"cwd": "/home/user/projects/myapp",
				"hook_event_name": "PostToolUse",
				"permission_mode": "plan",
				"tool_name": "Write",
				"tool_input": {
					"file_path": "/home/user/.claude/plans/my-plan.md"
				}
			}`,
			want: &Input{
				HookInput: cclocate.HookInput{
					SessionID:      "eb5b0174-0555-4601-804e-672d68069c89",
					TranscriptPath: "/home/user/.claude/projects/test/session.jsonl",
					CWD:            "/home/user/projects/myapp",
				},
				HookEventName:  "PostToolUse",
				PermissionMode: "plan",
				ToolName:       "Write",
				ToolInput:      &ToolInput{FilePath: "/home/user/.claude/plans/my-plan.md"},
			},
		},
		{
			name: "non-plan permission mode",
			json: `{
				"session_id": "test",
				"transcript_path": "/tmp/session.jsonl",
				"cwd": "/tmp",
				"hook_event_name": "PostToolUse",
				"permission_mode": "default",
				"tool_name": "Write",
				"tool_input": {
					"file_path": "/tmp/some-file.go"
				}
			}`,
			want: &Input{
				HookInput: cclocate.HookInput{
					SessionID:      "test",
					TranscriptPath: "/tmp/session.jsonl",
					CWD:            "/tmp",
				},
				HookEventName:  "PostToolUse",
				PermissionMode: "default",
				ToolName:       "Write",
				ToolInput:      &ToolInput{FilePath: "/tmp/some-file.go"},
			},
		},
		{
			name: "tool_input is null",
			json: `{
				"session_id": "test",
				"transcript_path": "/tmp/session.jsonl",
				"cwd": "/tmp",
				"hook_event_name": "PostToolUse",
				"permission_mode": "plan",
				"tool_name": "Write",
				"tool_input": null
			}`,
			want: &Input{
				HookInput: cclocate.HookInput{
					SessionID:      "test",
					TranscriptPath: "/tmp/session.jsonl",
					CWD:            "/tmp",
				},
				HookEventName:  "PostToolUse",
				PermissionMode: "plan",
				ToolName:       "Write",
				ToolInput:      nil,
			},
		},
		{
			name: "extra fields are ignored",
			json: `{
				"session_id": "test",
				"transcript_path": "/tmp/session.jsonl",
				"cwd": "/tmp",
				"hook_event_name": "PostToolUse",
				"permission_mode": "plan",
				"tool_name": "Write",
				"tool_input": {"file_path": "/tmp/plan.md"},
				"stop_hook_active": false,
				"tool_response": {"filePath": "/tmp/plan.md", "success": true},
				"tool_use_id": "toolu_01ABC123"
			}`,
			want: &Input{
				HookInput: cclocate.HookInput{
					SessionID:      "test",
					TranscriptPath: "/tmp/session.jsonl",
					CWD:            "/tmp",
				},
				HookEventName:  "PostToolUse",
				PermissionMode: "plan",
				ToolName:       "Write",
				ToolInput:      &ToolInput{FilePath: "/tmp/plan.md"},
			},
		},
		{
			name:    "invalid JSON",
			json:    `{broken`,
			wantErr: true,
		},
		{
			name:    "empty input",
			json:    ``,
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParseInput(strings.NewReader(tt.json))
			if (err != nil) != tt.wantErr {
				t.Fatalf("ParseInput() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.wantErr {
				return
			}
			if got.SessionID != tt.want.SessionID {
				t.Errorf("SessionID = %q, want %q", got.SessionID, tt.want.SessionID)
			}
			if got.TranscriptPath != tt.want.TranscriptPath {
				t.Errorf("TranscriptPath = %q, want %q", got.TranscriptPath, tt.want.TranscriptPath)
			}
			if got.CWD != tt.want.CWD {
				t.Errorf("CWD = %q, want %q", got.CWD, tt.want.CWD)
			}
			if got.HookEventName != tt.want.HookEventName {
				t.Errorf("HookEventName = %q, want %q", got.HookEventName, tt.want.HookEventName)
			}
			if got.PermissionMode != tt.want.PermissionMode {
				t.Errorf("PermissionMode = %q, want %q", got.PermissionMode, tt.want.PermissionMode)
			}
			if got.ToolName != tt.want.ToolName {
				t.Errorf("ToolName = %q, want %q", got.ToolName, tt.want.ToolName)
			}
			if tt.want.ToolInput == nil {
				if got.ToolInput != nil {
					t.Errorf("ToolInput = %v, want nil", got.ToolInput)
				}
			} else {
				if got.ToolInput == nil {
					t.Fatal("ToolInput = nil, want non-nil")
				}
				if got.ToolInput.FilePath != tt.want.ToolInput.FilePath {
					t.Errorf("ToolInput.FilePath = %q, want %q", got.ToolInput.FilePath, tt.want.ToolInput.FilePath)
				}
			}
		})
	}
}
