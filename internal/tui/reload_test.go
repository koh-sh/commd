package tui

import (
	"errors"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/koh-sh/commd/internal/markdown"
)

const reloadSource = "# Doc\n\nIntro.\n\n## Alpha\n\nalpha\n\n## Beta\n\nbeta\n"

func TestReloadKey(t *testing.T) {
	// A section added above shifts Beta from S2 to S3.
	inserted := strings.Replace(reloadSource, "## Alpha", "## New\n\nnew\n\n## Alpha", 1)
	tests := []struct {
		name         string
		noLoader     bool
		source       string // "" fails the read
		raw          bool   // reload in the raw view
		wantNotice   string
		wantSelected string // section selected after the reload
	}{
		{name: "changed file stays on the selected section", source: inserted, wantNotice: "Reloaded", wantSelected: "S3"},
		{name: "raw view stays raw", source: inserted, raw: true, wantNotice: "Reloaded", wantSelected: "S3"},
		{name: "unchanged file", source: reloadSource, wantNotice: "File unchanged", wantSelected: "S2"},
		{name: "failed read", wantNotice: "Cannot reload: gone", wantSelected: "S2"},
		{name: "no loader", noLoader: true, wantNotice: "Reload is not available for this file", wantSelected: "S2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var opts AppOptions
			if !tt.noLoader {
				opts.Load = func(path string) (markdown.File, error) {
					if tt.source == "" {
						return markdown.File{}, errors.New("gone")
					}
					return markdown.File{Path: path, Doc: mustParseDoc(t, tt.source)}, nil
				}
			}
			a := NewApp(markdown.File{Path: "doc.md", Doc: mustParseDoc(t, reloadSource)}, opts)
			a.Update(tea.WindowSizeMsg{Width: 120, Height: 30})
			a.sectionList.SelectBySectionID("S2")
			addTestComment(a, "S2", markdown.ReviewComment{Body: "on beta"})
			if tt.raw {
				a.Update(keyMsg("r"))
			}

			a.Update(keyMsg("R"))
			if a.notice != tt.wantNotice {
				t.Errorf("notice = %q, want %q", a.notice, tt.wantNotice)
			}
			if !strings.Contains(a.renderStatusBar(), tt.wantNotice) {
				t.Errorf("status bar = %q, want the notice", a.renderStatusBar())
			}
			if got := a.selectedSectionID(); got != tt.wantSelected {
				t.Errorf("selected = %q, want %q", got, tt.wantSelected)
			}
			if a.isRawMode() != tt.raw {
				t.Errorf("raw view = %v, want %v", a.isRawMode(), tt.raw)
			}
			if c := a.review.Comments(); len(c) != 1 || c[0].SectionID != tt.wantSelected {
				t.Errorf("comments = %+v, want one on %s", c, tt.wantSelected)
			}
			if a.Result().Doc != a.review.Doc {
				t.Error("the result does not carry the document the review shows")
			}

			a.Update(keyMsg("j"))
			if a.notice != "" {
				t.Errorf("notice = %q after the next key, want it cleared", a.notice)
			}
		})
	}
}

func mustParseDoc(t *testing.T, src string) *markdown.Document {
	t.Helper()
	doc, err := markdown.Parse([]byte(src))
	if err != nil {
		t.Fatal(err)
	}
	return doc
}
