package cchook

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/koh-sh/commd/internal/cclocate"
	"github.com/koh-sh/commd/internal/pane"
)

// mockSpawner implements pane.PaneSpawner for testing.
type mockSpawner struct {
	available   bool
	name        string
	spawnFunc   func(cmd string, args []string) error
	spawnCalled bool
}

func (m *mockSpawner) Available() bool { return m.available }
func (m *mockSpawner) Name() string    { return m.name }
func (m *mockSpawner) SpawnAndWait(_ context.Context, cmd string, args []string) error {
	m.spawnCalled = true
	if m.spawnFunc != nil {
		return m.spawnFunc(cmd, args)
	}
	return nil
}

// setupPlanEnv creates a temporary directory structure that simulates
// a project with .claude/settings.local.json pointing to a plans directory,
// and a plan file inside that directory.
func setupPlanEnv(t *testing.T) (plansDir, planFile, cwd string) {
	t.Helper()
	cwd = t.TempDir()
	plansDir = filepath.Join(cwd, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create settings.local.json pointing to the plans dir
	settingsDir := filepath.Join(cwd, ".claude")
	settings := map[string]string{"plansDirectory": plansDir}
	data, _ := json.Marshal(settings)
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.local.json"), data, 0o644); err != nil {
		t.Fatal(err)
	}

	// Create a plan file
	planFile = filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planFile, []byte("# Test Plan\n## S1\nStep content"), 0o644); err != nil {
		t.Fatal(err)
	}
	return
}

// postToolUseInput builds a plan-mode PostToolUse/Write hook input for filePath.
func postToolUseInput(cwd, filePath string) *Input {
	return &Input{
		HookInput:      cclocate.HookInput{CWD: cwd},
		HookEventName:  "PostToolUse",
		PermissionMode: "plan",
		ToolName:       "Write",
		ToolInput:      &ToolInput{FilePath: filePath},
	}
}

// outputPath returns the value following --output-path in the review args.
func outputPath(args []string) string {
	for i, arg := range args {
		if arg == "--output-path" && i+1 < len(args) {
			return args[i+1]
		}
	}
	return ""
}

// writeReview returns a spawnFunc that writes review to the --output-path
// file, simulating a review session that ended with that output.
func writeReview(review string) func(string, []string) error {
	return func(_ string, args []string) error {
		path := outputPath(args)
		if path == "" {
			return fmt.Errorf("--output-path not found in args")
		}
		return os.WriteFile(path, []byte(review), 0o644)
	}
}

func TestRunSkipsSpawn(t *testing.T) {
	plansDir, planFile, cwd := setupPlanEnv(t)
	notPlan := filepath.Join(t.TempDir(), "not-a-plan.go")
	if err := os.WriteFile(notPlan, nil, 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name    string
		input   *Input
		skipEnv string
	}{
		{name: "non-plan permission mode", input: &Input{PermissionMode: "default"}},
		{name: "CC_PLAN_REVIEW_SKIP=1", input: postToolUseInput(cwd, planFile), skipEnv: "1"},
		{name: "nil tool_input", input: &Input{PermissionMode: "plan"}},
		{name: "empty file_path", input: postToolUseInput(cwd, "")},
		{name: "file outside plans directory", input: postToolUseInput(cwd, notPlan)},
		{name: "plan file does not exist", input: postToolUseInput(cwd, filepath.Join(plansDir, "nonexistent.md"))},
		{
			name: "file_path without PostToolUse event",
			input: &Input{
				HookInput:      cclocate.HookInput{CWD: cwd},
				PermissionMode: "plan",
				ToolInput:      &ToolInput{FilePath: planFile},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Setenv("CC_PLAN_REVIEW_SKIP", tt.skipEnv)
			mock := &mockSpawner{available: true, name: "mock"}
			code := Run(context.Background(), tt.input, RunConfig{Spawner: mock})
			if code != 0 {
				t.Errorf("exit code = %d, want 0", code)
			}
			if mock.spawnCalled {
				t.Error("spawner should not be called")
			}
		})
	}
}

func TestRunSpawnOutcomes(t *testing.T) {
	_, planFile, cwd := setupPlanEnv(t)

	tests := []struct {
		name      string
		spawner   string // spawner name; NameDirect disables the direct fallback
		spawnFunc func(cmd string, args []string) error
		wantCode  int
	}{
		{name: "submitted review returns 2", spawnFunc: writeReview("review feedback"), wantCode: 2},
		{name: "empty review returns 0", spawnFunc: writeReview(""), wantCode: 0},
		{
			name:      "spawn failure without fallback returns 0",
			spawner:   pane.NameDirect,
			spawnFunc: func(string, []string) error { return fmt.Errorf("spawn failed") },
			wantCode:  0,
		},
		{
			name: "review file removed returns 0",
			spawnFunc: func(_ string, args []string) error {
				os.Remove(outputPath(args))
				return nil
			},
			wantCode: 0,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			name := tt.spawner
			if name == "" {
				name = "mock"
			}
			mock := &mockSpawner{available: true, name: name, spawnFunc: tt.spawnFunc}
			code := Run(context.Background(), postToolUseInput(cwd, planFile), RunConfig{Spawner: mock})
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !mock.spawnCalled {
				t.Error("spawner should have been called")
			}
		})
	}
}

func TestResolvePlanFile(t *testing.T) {
	_, planFile, cwd := setupPlanEnv(t)
	outside := filepath.Join(t.TempDir(), "elsewhere.md")

	tests := []struct {
		name   string
		input  *Input
		want   string
		wantOK bool
	}{
		{
			name:   "nil tool_input",
			input:  &Input{HookInput: cclocate.HookInput{CWD: cwd}},
			wantOK: false,
		},
		{
			name: "PreToolUse ExitPlanMode uses planFilePath without plansDirectory lookup",
			input: &Input{
				HookInput:     cclocate.HookInput{CWD: cwd},
				HookEventName: "PreToolUse",
				ToolName:      "ExitPlanMode",
				ToolInput:     &ToolInput{PlanFilePath: outside},
			},
			want:   outside,
			wantOK: true,
		},
		{
			name: "PreToolUse ExitPlanMode with empty planFilePath",
			input: &Input{
				HookInput:     cclocate.HookInput{CWD: cwd},
				HookEventName: "PreToolUse",
				ToolName:      "ExitPlanMode",
				ToolInput:     &ToolInput{},
			},
			wantOK: false,
		},
		{
			name: "PreToolUse ExitPlanMode ignores file_path",
			input: &Input{
				HookInput:     cclocate.HookInput{CWD: cwd},
				HookEventName: "PreToolUse",
				ToolName:      "ExitPlanMode",
				ToolInput:     &ToolInput{FilePath: planFile},
			},
			wantOK: false,
		},
		{
			name: "PreToolUse on another tool ignores file_path",
			input: &Input{
				HookInput:     cclocate.HookInput{CWD: cwd},
				HookEventName: "PreToolUse",
				ToolName:      "Write",
				ToolInput:     &ToolInput{FilePath: planFile},
			},
			wantOK: false,
		},
		{
			name:   "PostToolUse Write under plansDirectory",
			input:  postToolUseInput(cwd, planFile),
			want:   planFile,
			wantOK: true,
		},
		{
			name:   "PostToolUse Write outside plansDirectory",
			input:  postToolUseInput(cwd, outside),
			wantOK: false,
		},
		{
			name:   "PostToolUse Write with empty file_path",
			input:  postToolUseInput(cwd, ""),
			wantOK: false,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := resolvePlanFile(tt.input)
			if ok != tt.wantOK {
				t.Fatalf("ok = %v, want %v (got %q)", ok, tt.wantOK, got)
			}
			if ok && got != tt.want {
				t.Errorf("planFile = %q, want %q", got, tt.want)
			}
		})
	}
}

// TestRunExitPlanModeTrigger verifies the PreToolUse/ExitPlanMode path end to
// end: the injected planFilePath is reviewed even though it is not under the
// resolved plansDirectory, and a submitted review yields exit code 2.
func TestRunExitPlanModeTrigger(t *testing.T) {
	planFile := filepath.Join(t.TempDir(), "plan.md")
	if err := os.WriteFile(planFile, []byte("# Plan\n## S1\nStep"), 0o644); err != nil {
		t.Fatal(err)
	}

	tests := []struct {
		name     string
		review   string
		wantCode int
	}{
		{name: "submitted review returns 2", review: "review feedback", wantCode: 2},
		{name: "approved (empty review) returns 0", review: "", wantCode: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var reviewedFile string
			write := writeReview(tt.review)
			mock := &mockSpawner{
				available: true,
				name:      "mock",
				spawnFunc: func(cmd string, args []string) error {
					reviewedFile = args[len(args)-1]
					return write(cmd, args)
				},
			}
			input := &Input{
				HookInput:      cclocate.HookInput{CWD: t.TempDir()},
				HookEventName:  "PreToolUse",
				PermissionMode: "plan",
				ToolName:       "ExitPlanMode",
				ToolInput:      &ToolInput{PlanFilePath: planFile},
			}
			code := Run(context.Background(), input, RunConfig{Spawner: mock, Theme: "dark"})
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
			if !mock.spawnCalled {
				t.Fatal("spawner should be called for ExitPlanMode trigger")
			}
			if reviewedFile != planFile {
				t.Errorf("reviewed file = %q, want %q", reviewedFile, planFile)
			}
		})
	}
}
