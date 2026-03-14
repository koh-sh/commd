package cchook

import (
	"encoding/json"
	"fmt"
	"io"

	"github.com/koh-sh/commd/internal/cclocate"
)

// permissionModePlan is the Claude Code permission mode that triggers plan review.
const permissionModePlan = "plan"

// Input represents the JSON input from a Claude Code PostToolUse hook.
// It embeds cclocate.HookInput for the common fields (session_id, transcript_path, cwd).
type Input struct {
	cclocate.HookInput
	HookEventName  string     `json:"hook_event_name"`
	PermissionMode string     `json:"permission_mode"`
	ToolName       string     `json:"tool_name"`
	ToolInput      *ToolInput `json:"tool_input"`
}

// ToolInput represents the input parameters of a Write tool call.
type ToolInput struct {
	FilePath string `json:"file_path"`
}

// ParseInput reads and parses hook JSON input from a reader.
func ParseInput(r io.Reader) (*Input, error) {
	data, err := io.ReadAll(r)
	if err != nil {
		return nil, fmt.Errorf("reading input: %w", err)
	}

	var input Input
	if err := json.Unmarshal(data, &input); err != nil {
		return nil, fmt.Errorf("parsing hook input JSON: %w", err)
	}

	return &input, nil
}
