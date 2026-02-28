package pane

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"time"
)

const (
	pollInterval = 500 * time.Millisecond
	maxWaitTime  = 10 * time.Minute
)

// cmdRunner abstracts command execution for testing.
type cmdRunner interface {
	Output(name string, args ...string) ([]byte, error)
}

// WezTermSpawner spawns commands in a new WezTerm pane.
type WezTermSpawner struct {
	runner cmdRunner
}

func (w *WezTermSpawner) run(name string, args ...string) ([]byte, error) {
	if w.runner != nil {
		return w.runner.Output(name, args...)
	}
	return exec.Command(name, args...).Output()
}

func (w *WezTermSpawner) Available() bool {
	_, err := exec.LookPath("wezterm")
	return err == nil
}

func (w *WezTermSpawner) Name() string {
	return NameWezTerm
}

func (w *WezTermSpawner) SpawnAndWait(ctx context.Context, cmd string, args []string) error {
	direction, percent := w.splitDirection()

	fullCmd := append([]string{cmd}, args...)
	splitArgs := append([]string{
		"cli", "split-pane", direction, "--percent", percent, "--",
	}, fullCmd...)

	out, err := w.run("wezterm", splitArgs...)
	if err != nil {
		return fmt.Errorf("wezterm split-pane: %w", err)
	}
	paneID := strings.TrimSpace(string(out))

	timeout := time.After(maxWaitTime)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-timeout:
			return fmt.Errorf("timeout waiting for pane %s to close", paneID)
		case <-ticker.C:
			if !w.paneExists(paneID) {
				return nil
			}
		}
	}
}

type paneSize struct {
	Rows        int `json:"rows"`
	Cols        int `json:"cols"`
	PixelWidth  int `json:"pixel_width"`
	PixelHeight int `json:"pixel_height"`
}

type weztermPane struct {
	PaneID json.Number `json:"pane_id"`
	Size   paneSize    `json:"size"`
}

// listPanes returns all WezTerm panes by calling wezterm cli list.
func (w *WezTermSpawner) listPanes() ([]weztermPane, error) {
	out, err := w.run("wezterm", "cli", "list", "--format", "json")
	if err != nil {
		return nil, fmt.Errorf("wezterm cli list: %w", err)
	}
	var panes []weztermPane
	if err := json.Unmarshal(out, &panes); err != nil {
		return nil, fmt.Errorf("parse json: %w", err)
	}
	return panes, nil
}

// findPane returns the pane with the given ID, or nil if not found.
func (w *WezTermSpawner) findPane(paneID string) (*weztermPane, error) {
	panes, err := w.listPanes()
	if err != nil {
		return nil, err
	}
	for _, p := range panes {
		if p.PaneID.String() == paneID {
			return &p, nil
		}
	}
	return nil, nil
}

// splitDirection returns the split direction and percent based on current pane dimensions.
// Uses pixel dimensions for accurate aspect ratio detection.
// Wide panes (width > height) split right at 50%, tall/square panes split bottom at 80%.
func (w *WezTermSpawner) splitDirection() (string, string) {
	size, err := w.currentPaneSize()
	if err != nil {
		return "--bottom", "80"
	}
	if size.PixelWidth > 0 && size.PixelHeight > 0 {
		if size.PixelWidth > size.PixelHeight {
			return "--right", "50"
		}
		return "--bottom", "80"
	}
	// Pixel dimensions unavailable; fall back to safe default.
	// Cell-based heuristics are unreliable because the cell aspect ratio
	// varies by font (typically 1:2.3–2.5, not the 1:2 often assumed).
	return "--bottom", "80"
}

// currentPaneSize returns the dimensions of the current pane using WEZTERM_PANE env var.
func (w *WezTermSpawner) currentPaneSize() (*paneSize, error) {
	currentPaneID := os.Getenv("WEZTERM_PANE")
	if currentPaneID == "" {
		return nil, fmt.Errorf("WEZTERM_PANE not set")
	}
	p, err := w.findPane(currentPaneID)
	if err != nil {
		return nil, err
	}
	if p == nil {
		return nil, fmt.Errorf("pane %s not found", currentPaneID)
	}
	return &p.Size, nil
}

func (w *WezTermSpawner) paneExists(paneID string) bool {
	p, err := w.findPane(paneID)
	return err == nil && p != nil
}
