package pane

import (
	"context"
	"fmt"
	"testing"
)

func TestWezTermSpawnerName(t *testing.T) {
	w := &WezTermSpawner{}
	if w.Name() != "wezterm" {
		t.Errorf("name = %s, want wezterm", w.Name())
	}
}

func TestWezTermSpawnerAvailable(t *testing.T) {
	w := &WezTermSpawner{}
	// Just verify it doesn't panic; result depends on environment
	_ = w.Available()
}

// mockRunner implements cmdRunner for testing.
type mockRunner struct {
	calls   []mockCall
	callIdx int
}

type mockCall struct {
	out []byte
	err error
}

func (m *mockRunner) Output(name string, args ...string) ([]byte, error) {
	if m.callIdx >= len(m.calls) {
		return nil, fmt.Errorf("unexpected call #%d to Output(%s)", m.callIdx, name)
	}
	c := m.calls[m.callIdx]
	m.callIdx++
	return c.out, c.err
}

func TestSplitDirectionWidePixel(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "0")
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`[{"pane_id":0,"size":{"rows":40,"cols":120,"pixel_width":1920,"pixel_height":1080}}]`)},
			},
		},
	}
	dir, pct := w.splitDirection()
	if dir != "--right" || pct != "50" {
		t.Errorf("splitDirection() = %s %s, want --right 50", dir, pct)
	}
}

func TestSplitDirectionTallPixel(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "0")
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`[{"pane_id":0,"size":{"rows":80,"cols":60,"pixel_width":800,"pixel_height":1200}}]`)},
			},
		},
	}
	dir, pct := w.splitDirection()
	if dir != "--bottom" || pct != "80" {
		t.Errorf("splitDirection() = %s %s, want --bottom 80", dir, pct)
	}
}

func TestSplitDirectionPixelUnavailableFallback(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "0")
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`[{"pane_id":0,"size":{"rows":40,"cols":200,"pixel_width":0,"pixel_height":0}}]`)},
			},
		},
	}
	dir, pct := w.splitDirection()
	if dir != "--bottom" || pct != "80" {
		t.Errorf("splitDirection() = %s %s, want --bottom 80 (safe default when pixels unavailable)", dir, pct)
	}
}

func TestSplitDirectionErrorFallback(t *testing.T) {
	// No WEZTERM_PANE set → error → fallback
	t.Setenv("WEZTERM_PANE", "")
	w := &WezTermSpawner{}
	dir, pct := w.splitDirection()
	if dir != "--bottom" || pct != "80" {
		t.Errorf("splitDirection() = %s %s, want --bottom 80", dir, pct)
	}
}

func TestCurrentPaneSizeNoEnv(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "")
	w := &WezTermSpawner{}
	_, err := w.currentPaneSize()
	if err == nil {
		t.Error("expected error when WEZTERM_PANE not set")
	}
}

func TestCurrentPaneSizeCmdError(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "0")
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{err: fmt.Errorf("command not found")},
			},
		},
	}
	_, err := w.currentPaneSize()
	if err == nil {
		t.Error("expected error on command failure")
	}
}

func TestCurrentPaneSizeInvalidJSON(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "0")
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`not json`)},
			},
		},
	}
	_, err := w.currentPaneSize()
	if err == nil {
		t.Error("expected error on invalid JSON")
	}
}

func TestCurrentPaneSizePaneNotFound(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "99")
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`[{"pane_id":0,"size":{"rows":40,"cols":120}}]`)},
			},
		},
	}
	_, err := w.currentPaneSize()
	if err == nil {
		t.Error("expected error when pane not found")
	}
}

func TestCurrentPaneSizeSuccess(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "5")
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`[{"pane_id":5,"size":{"rows":40,"cols":120,"pixel_width":1920,"pixel_height":1080}}]`)},
			},
		},
	}
	size, err := w.currentPaneSize()
	if err != nil {
		t.Fatal(err)
	}
	if size.Rows != 40 || size.Cols != 120 || size.PixelWidth != 1920 || size.PixelHeight != 1080 {
		t.Errorf("size = %+v, unexpected", size)
	}
}

func TestPaneExistsTrue(t *testing.T) {
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`[{"pane_id":42,"size":{"rows":40,"cols":120}}]`)},
			},
		},
	}
	if !w.paneExists("42") {
		t.Error("paneExists(42) = false, want true")
	}
}

func TestPaneExistsFalse(t *testing.T) {
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`[{"pane_id":1,"size":{"rows":40,"cols":120}}]`)},
			},
		},
	}
	if w.paneExists("99") {
		t.Error("paneExists(99) = true, want false")
	}
}

func TestPaneExistsCmdError(t *testing.T) {
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{err: fmt.Errorf("command failed")},
			},
		},
	}
	if w.paneExists("0") {
		t.Error("paneExists should return false on command error")
	}
}

func TestPaneExistsInvalidJSON(t *testing.T) {
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{out: []byte(`invalid`)},
			},
		},
	}
	if w.paneExists("0") {
		t.Error("paneExists should return false on invalid JSON")
	}
}

func TestSpawnAndWaitSuccess(t *testing.T) {
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				// splitDirection → currentPaneSize (WEZTERM_PANE not set → fallback)
				// SpawnAndWait → split-pane returns pane ID
				{out: []byte("42\n")},
				// First poll: pane gone
				{out: []byte(`[]`)},
			},
		},
	}
	// Ensure WEZTERM_PANE is not set so splitDirection falls back without calling runner
	t.Setenv("WEZTERM_PANE", "")
	err := w.SpawnAndWait(context.Background(), "commd", []string{"review", "plan.md"})
	if err != nil {
		t.Fatalf("SpawnAndWait() error = %v", err)
	}
}

func TestSpawnAndWaitSplitError(t *testing.T) {
	t.Setenv("WEZTERM_PANE", "")
	w := &WezTermSpawner{
		runner: &mockRunner{
			calls: []mockCall{
				{err: fmt.Errorf("split failed")},
			},
		},
	}
	err := w.SpawnAndWait(context.Background(), "commd", []string{"review"})
	if err == nil {
		t.Error("expected error on split-pane failure")
	}
}
