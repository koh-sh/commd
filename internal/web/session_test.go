package web

import (
	"errors"
	"slices"
	"strings"
	"testing"

	"github.com/koh-sh/commd/internal/markdown"
)

// testLoader loads the paths it knows (as testSource documents) and fails
// the others, recording the order of loads.
type testLoader struct {
	t      *testing.T
	known  []string
	loaded []string
}

func (l *testLoader) load(path string) (markdown.File, error) {
	l.loaded = append(l.loaded, path)
	if !slices.Contains(l.known, path) {
		return markdown.File{}, errors.New("unknown")
	}
	return markdown.File{Path: path, Doc: mustParse(l.t, testSource)}, nil
}

// step is one action in a session flow test.
type step struct {
	pick    []string // pick these paths; empty (not nil) cancels the picker
	comment bool     // add a section comment to the current file
	finish  string   // "submit" or "quit" the current file
	stale   bool     // finish with an outdated seq, expecting errStale
}

func TestSessionFlow(t *testing.T) {
	type fileOutcome struct {
		path     string
		status   markdown.Status
		comments int
	}
	tests := []struct {
		name       string
		review     Review
		known      []string
		steps      []step
		wantLoaded []string
		want       []fileOutcome
	}{
		{
			name:       "single file submitted",
			review:     Review{Paths: []string{"a.md"}},
			known:      []string{"a.md"},
			steps:      []step{{comment: true}, {finish: "submit"}},
			wantLoaded: []string{"a.md"},
			want:       []fileOutcome{{"a.md", markdown.StatusSubmitted, 1}},
		},
		{
			name:       "single file approved without comments",
			review:     Review{Paths: []string{"a.md"}},
			known:      []string{"a.md"},
			steps:      []step{{finish: "submit"}},
			wantLoaded: []string{"a.md"},
			want:       []fileOutcome{{"a.md", markdown.StatusApproved, 0}},
		},
		{
			name:       "files are loaded one at a time, quit skips a file",
			review:     Review{Paths: []string{"a.md", "b.md"}},
			known:      []string{"a.md", "b.md"},
			steps:      []step{{comment: true}, {finish: "quit"}, {finish: "submit"}},
			wantLoaded: []string{"a.md", "b.md"},
			want:       []fileOutcome{{"a.md", markdown.StatusCancelled, 0}, {"b.md", markdown.StatusApproved, 0}},
		},
		{
			name:       "unloadable files are skipped",
			review:     Review{Paths: []string{"gone.md", "b.md"}},
			known:      []string{"b.md"},
			steps:      []step{{finish: "submit"}},
			wantLoaded: []string{"gone.md", "b.md"},
			want:       []fileOutcome{{"b.md", markdown.StatusApproved, 0}},
		},
		{
			name:       "picked files are reviewed in picker order",
			review:     Review{Pick: []string{"a.md", "b.md", "c.md"}},
			known:      []string{"a.md", "b.md", "c.md"},
			steps:      []step{{pick: []string{"c.md", "a.md"}}, {finish: "submit"}, {finish: "submit"}},
			wantLoaded: []string{"a.md", "c.md"},
			want:       []fileOutcome{{"a.md", markdown.StatusApproved, 0}, {"c.md", markdown.StatusApproved, 0}},
		},
		{
			name:   "unknown picked paths are ignored",
			review: Review{Pick: []string{"a.md"}},
			known:  []string{"a.md"},
			steps:  []step{{pick: []string{"../etc/passwd"}}},
		},
		{
			name:   "cancelled picker reviews nothing",
			review: Review{Pick: []string{"a.md"}},
			known:  []string{"a.md"},
			steps:  []step{{pick: []string{}}},
		},
		{
			name:       "a request for a finished file is stale",
			review:     Review{Paths: []string{"a.md", "b.md"}},
			known:      []string{"a.md", "b.md"},
			steps:      []step{{finish: "submit"}, {stale: true}, {finish: "submit"}},
			wantLoaded: []string{"a.md", "b.md"},
			want:       []fileOutcome{{"a.md", markdown.StatusApproved, 0}, {"b.md", markdown.StatusApproved, 0}},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			loader := &testLoader{t: t, known: tt.known}
			tt.review.Load = loader.load
			s := newSession(tt.review, testToken, Options{Theme: "light"})
			for _, st := range tt.steps {
				var err error
				switch {
				case st.pick != nil:
					err = s.pick(st.pick)
				case st.comment:
					err = s.withFile(s.seq, func(f *fileState) error {
						return f.addComment(markdown.ReviewComment{SectionID: "S1", Action: markdown.ActionNote, Body: "b"})
					})
				case st.stale:
					if err := s.finish(s.seq-1, true); !errors.Is(err, errStale) {
						t.Fatalf("stale finish: err = %v, want %v", err, errStale)
					}
				default:
					err = s.finish(s.seq, st.finish == "submit")
				}
				if err != nil {
					t.Fatalf("step %+v: %v", st, err)
				}
			}
			if s.phase != phaseDone {
				t.Fatalf("phase = %s, want %s", s.phase, phaseDone)
			}
			res := <-s.done
			var got []fileOutcome
			for _, f := range res {
				n := 0
				if f.Review != nil {
					n = len(f.Review.Comments)
				}
				got = append(got, fileOutcome{f.Path, f.Status, n})
			}
			if !slices.Equal(got, tt.want) {
				t.Errorf("results = %v, want %v", got, tt.want)
			}
			if !slices.Equal(loader.loaded, tt.wantLoaded) {
				t.Errorf("loaded = %v, want %v", loader.loaded, tt.wantLoaded)
			}
			// Nothing can change once the session ended.
			if err := s.finish(s.seq, true); !errors.Is(err, errFinished) {
				t.Errorf("finish after the end: err = %v, want %v", err, errFinished)
			}
		})
	}
}

func TestSessionPhaseErrors(t *testing.T) {
	picking := func(t *testing.T) *session {
		return newSession(Review{Pick: []string{"a.md"}, Load: (&testLoader{t: t, known: []string{"a.md"}}).load}, testToken, Options{Theme: "light"})
	}
	reviewing := func(t *testing.T) *session {
		return newSession(Review{Paths: []string{"a.md"}, Load: (&testLoader{t: t, known: []string{"a.md"}}).load}, testToken, Options{Theme: "light"})
	}
	tests := []struct {
		name  string
		start func(t *testing.T) *session
		run   func(s *session) error
	}{
		{name: "comment while picking", start: picking, run: func(s *session) error {
			return s.withFile(s.seq, func(*fileState) error { return nil })
		}},
		{name: "finish while picking", start: picking, run: func(s *session) error { return s.finish(s.seq, true) }},
		{name: "pick while reviewing", start: reviewing, run: func(s *session) error { return s.pick([]string{"a.md"}) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(tt.start(t)); !errors.Is(err, errPhase) {
				t.Errorf("err = %v, want %v", err, errPhase)
			}
		})
	}
}

func TestSessionReload(t *testing.T) {
	// The heading of the commented section is renamed on disk.
	edited := strings.Replace(testSource, "## First", "## Renamed", 1)
	tests := []struct {
		name       string
		pick       bool   // the session is still in the picker
		source     string // "" fails the load
		wantSeq    int
		wantTitle  string // title of the first section after the reload
		wantResult string // the reload message; "" when there is nothing to reload
	}{
		{name: "reads the file again", source: edited, wantSeq: 2, wantTitle: "Renamed", wantResult: "Reloaded; 1 comments no longer match the file and were moved to a section"},
		{name: "an unchanged file is not reloaded", source: testSource, wantSeq: 1, wantTitle: "First", wantResult: "File unchanged"},
		{name: "a failed read keeps the last content", wantSeq: 1, wantTitle: "First", wantResult: "Cannot reload: gone"},
		{name: "nothing to read while picking", pick: true, source: edited, wantSeq: 0},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			source := testSource
			load := func(path string) (markdown.File, error) {
				if source == "" {
					return markdown.File{}, errors.New("gone")
				}
				return markdown.File{Path: path, Doc: mustParse(t, source)}, nil
			}
			review := Review{Paths: []string{"a.md"}, Load: load}
			if tt.pick {
				review = Review{Pick: []string{"a.md"}, Load: load}
			}
			s := newSession(review, testToken, Options{Theme: "light"})
			if !tt.pick {
				err := s.withFile(s.seq, func(f *fileState) error {
					return f.addComment(markdown.ReviewComment{SectionID: "S1", Action: markdown.ActionNote, Body: "b"})
				})
				if err != nil {
					t.Fatal(err)
				}
			}

			source = tt.source
			res, ok := s.reload()
			if got := res.Message(); ok != (tt.wantResult != "") || ok && got != tt.wantResult {
				t.Errorf("reload = %q (ok %v), want %q", got, ok, tt.wantResult)
			}
			state := s.state()
			if state.Seq != tt.wantSeq {
				t.Errorf("seq = %d, want %d", state.Seq, tt.wantSeq)
			}
			if tt.pick {
				return
			}
			if got := state.File.Sections[1].Title; got != tt.wantTitle {
				t.Errorf("first section = %q, want %q", got, tt.wantTitle)
			}
			if len(state.File.Comments) != 1 {
				t.Errorf("got %d comments, want 1 kept", len(state.File.Comments))
			}
		})
	}
}
