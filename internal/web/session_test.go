package web

import (
	"errors"
	"slices"
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

func (l *testLoader) load(path string) (markdown.File, bool) {
	l.loaded = append(l.loaded, path)
	if !slices.Contains(l.known, path) {
		return markdown.File{}, false
	}
	return markdown.File{Path: path, Doc: mustParse(l.t, testSource)}, true
}

// step is one action in a session flow test.
type step struct {
	pick    []string // pick these paths
	cancel  bool     // cancel the picker
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
			steps:  []step{{cancel: true}},
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
			s := newSession(tt.review, testToken, "light")
			for _, st := range tt.steps {
				var err error
				switch {
				case st.pick != nil || st.cancel:
					err = s.pick(st.pick, st.cancel)
				case st.comment:
					err = s.withFile(s.seq, func(f *fileState) error {
						return f.addComment(commentInput{SectionID: "S1", Action: "note", Body: "b"})
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
			for _, f := range res.Files {
				n := 0
				if f.Review != nil {
					n = len(f.Review.Comments)
				}
				got = append(got, fileOutcome{f.File.Path, f.Status, n})
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
		return newSession(Review{Pick: []string{"a.md"}, Load: (&testLoader{t: t, known: []string{"a.md"}}).load}, testToken, "light")
	}
	reviewing := func(t *testing.T) *session {
		return newSession(Review{Paths: []string{"a.md"}, Load: (&testLoader{t: t, known: []string{"a.md"}}).load}, testToken, "light")
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
		{name: "pick while reviewing", start: reviewing, run: func(s *session) error { return s.pick([]string{"a.md"}, false) }},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if err := tt.run(tt.start(t)); !errors.Is(err, errPhase) {
				t.Errorf("err = %v, want %v", err, errPhase)
			}
		})
	}
}
