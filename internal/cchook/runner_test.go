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

func TestRunSkipsNonPlanMode(t *testing.T) {
	mock := &mockSpawner{available: true, name: "mock"}
	input := &Input{PermissionMode: "default"}
	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if mock.spawnCalled {
		t.Error("spawner should not be called for non-plan mode")
	}
}

func TestRunSkipsPlanReviewSkipEnv(t *testing.T) {
	t.Setenv("CC_PLAN_REVIEW_SKIP", "1")
	mock := &mockSpawner{available: true, name: "mock"}
	input := &Input{PermissionMode: "plan"}
	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if mock.spawnCalled {
		t.Error("spawner should not be called when CC_PLAN_REVIEW_SKIP=1")
	}
}

func TestRunSkipsNilToolInput(t *testing.T) {
	mock := &mockSpawner{available: true, name: "mock"}
	input := &Input{
		PermissionMode: "plan",
		ToolInput:      nil,
	}
	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if mock.spawnCalled {
		t.Error("spawner should not be called with nil tool_input")
	}
}

func TestRunSkipsEmptyFilePath(t *testing.T) {
	mock := &mockSpawner{available: true, name: "mock"}
	input := &Input{
		PermissionMode: "plan",
		ToolInput:      &ToolInput{FilePath: ""},
	}
	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if mock.spawnCalled {
		t.Error("spawner should not be called with empty file_path")
	}
}

func TestRunSkipsNonPlanFile(t *testing.T) {
	// Create a temp file that exists but is NOT under plans directory
	tmpFile, err := os.CreateTemp("", "not-a-plan-*.go")
	if err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()
	defer os.Remove(tmpFile.Name())

	mock := &mockSpawner{available: true, name: "mock"}
	input := &Input{
		HookInput:      cclocate.HookInput{CWD: t.TempDir()},
		PermissionMode: "plan",
		ToolInput:      &ToolInput{FilePath: tmpFile.Name()},
	}
	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if mock.spawnCalled {
		t.Error("spawner should not be called for file outside plans directory")
	}
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

func TestRunSuccessWithReview(t *testing.T) {
	_, planFile, cwd := setupPlanEnv(t)

	mock := &mockSpawner{
		available: true,
		name:      "mock",
		spawnFunc: func(cmd string, args []string) error {
			// Find --output-path in args and write review content
			for i, arg := range args {
				if arg == "--output-path" && i+1 < len(args) {
					return os.WriteFile(args[i+1], []byte("review feedback"), 0o644)
				}
			}
			return fmt.Errorf("--output-path not found in args")
		},
	}

	input := &Input{
		HookInput:      cclocate.HookInput{CWD: cwd},
		PermissionMode: "plan",
		ToolInput:      &ToolInput{FilePath: planFile},
	}

	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 2 {
		t.Errorf("exit code = %d, want 2 (feedback)", code)
	}
	if !mock.spawnCalled {
		t.Error("spawner should have been called")
	}
}

func TestRunSuccessNoReview(t *testing.T) {
	_, planFile, cwd := setupPlanEnv(t)

	mock := &mockSpawner{
		available: true,
		name:      "mock",
		spawnFunc: func(cmd string, args []string) error {
			// Don't write anything to output path (empty review)
			return nil
		},
	}

	input := &Input{
		HookInput:      cclocate.HookInput{CWD: cwd},
		PermissionMode: "plan",
		ToolInput:      &ToolInput{FilePath: planFile},
	}

	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunFileNotFound(t *testing.T) {
	plansDir, _, cwd := setupPlanEnv(t)

	mock := &mockSpawner{available: true, name: "mock"}
	input := &Input{
		HookInput:      cclocate.HookInput{CWD: cwd},
		PermissionMode: "plan",
		ToolInput:      &ToolInput{FilePath: filepath.Join(plansDir, "nonexistent.md")},
	}

	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
	if mock.spawnCalled {
		t.Error("spawner should not be called for nonexistent file")
	}
}

func TestRunSpawnFailure(t *testing.T) {
	_, planFile, cwd := setupPlanEnv(t)

	mock := &mockSpawner{
		available: true,
		name:      pane.NameDirect, // name="direct" so fallback is skipped
		spawnFunc: func(cmd string, args []string) error {
			return fmt.Errorf("spawn failed")
		},
	}

	input := &Input{
		HookInput:      cclocate.HookInput{CWD: cwd},
		PermissionMode: "plan",
		ToolInput:      &ToolInput{FilePath: planFile},
	}

	// When spawn fails and name is "direct", no further fallback → exit 0
	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}

func TestRunReviewFileRemoved(t *testing.T) {
	_, planFile, cwd := setupPlanEnv(t)

	mock := &mockSpawner{
		available: true,
		name:      "mock",
		spawnFunc: func(cmd string, args []string) error {
			// Remove the review output file to trigger ReadFile error
			for i, arg := range args {
				if arg == "--output-path" && i+1 < len(args) {
					os.Remove(args[i+1])
					return nil
				}
			}
			return nil
		},
	}

	input := &Input{
		HookInput:      cclocate.HookInput{CWD: cwd},
		PermissionMode: "plan",
		ToolInput:      &ToolInput{FilePath: planFile},
	}

	code, err := Run(input, RunConfig{Spawner: mock})
	if err != nil {
		t.Fatal(err)
	}
	if code != 0 {
		t.Errorf("exit code = %d, want 0", code)
	}
}
