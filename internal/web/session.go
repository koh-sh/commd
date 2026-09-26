package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"sync"

	"github.com/koh-sh/commd/internal/markdown"
)

// Review describes what a browser session reviews. Like the TUI, files are
// reviewed one at a time, and each is loaded only when its turn comes so that
// edits made while reviewing an earlier file are picked up.
type Review struct {
	// Pick, when set, is offered in a file picker first (all selected, like
	// the TUI picker); the chosen files are then reviewed in this order.
	Pick []string
	// Paths are reviewed in order when Pick is empty.
	Paths []string
	// Load reads a file when its turn comes. ok=false skips it; Load reports
	// the reason itself.
	Load func(path string) (f File, ok bool)
}

// FileResult is the outcome of reviewing one file.
type FileResult struct {
	File   File
	Status markdown.Status
	Review *markdown.ReviewResult // nil when the file was quit (skipped)
}

// Result is the outcome of a browser session: one entry per file that was
// loaded, in review order. It is empty when the picker was cancelled or the
// session was interrupted.
type Result struct {
	Files []FileResult
}

// Session phases, as reported to the browser.
const (
	phasePick   = "pick"
	phaseReview = "review"
	phaseDone   = "done"
)

// errFinished is returned for any change after the session ended.
var errFinished = errors.New("review already finished")

// errNotFound marks lookups of unknown comments or sections.
var errNotFound = errors.New("not found")

// errStale rejects a request for a file that is no longer under review, e.g.
// from a second tab that missed a finish.
var errStale = errors.New("that file is no longer under review; reload the page")

// errPhase rejects a request that does not fit the current phase, e.g. a
// comment while the picker is shown.
var errPhase = errors.New("not possible in the current phase")

// session holds the state of a browser review. All methods are safe for
// concurrent use by HTTP handlers.
type session struct {
	mu      sync.Mutex
	review  Review
	phase   string
	multi   bool     // several files were chosen: dialogs say finish/skip this file
	queue   []string // files still to review after the current one
	current *fileState
	seq     int // increments per reviewed file; requests carry it to detect stale pages
	results []FileResult
	done    chan Result // receives the result once when the session ends
}

func newSession(review Review) *session {
	s := &session{review: review, done: make(chan Result, 1)}
	if len(review.Pick) > 0 {
		s.phase = phasePick
	} else {
		s.start(review.Paths)
	}
	return s
}

// start begins reviewing paths in order.
func (s *session) start(paths []string) {
	s.multi = len(paths) > 1
	s.queue = paths
	s.advance()
}

// advance loads the next reviewable file, or ends the session when none is
// left.
func (s *session) advance() {
	s.current = nil
	for len(s.queue) > 0 {
		path := s.queue[0]
		s.queue = s.queue[1:]
		if f, ok := s.review.Load(path); ok {
			s.current = newFileState(f)
			s.seq++
			s.phase = phaseReview
			return
		}
	}
	s.phase = phaseDone
	s.done <- Result{Files: s.results}
}

// pick starts the review of the chosen files, keeping the picker's order.
// Choosing nothing or cancelling ends the session without reviews.
func (s *session) pick(paths []string, cancel bool) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != phasePick {
		return s.phaseError()
	}
	var chosen []string
	for _, p := range s.review.Pick {
		if slices.Contains(paths, p) {
			chosen = append(chosen, p)
		}
	}
	if cancel {
		chosen = nil
	}
	s.start(chosen)
	return nil
}

// withFile runs fn on the file under review if seq still names it.
func (s *session) withFile(seq int, fn func(f *fileState) error) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.phase != phaseReview {
		return s.phaseError()
	}
	if seq != s.seq {
		return errStale
	}
	return fn(s.current)
}

// finish ends the review of the current file: submit records its comments
// (approved when there are none), otherwise it is skipped. The next file is
// loaded, or the session ends.
func (s *session) finish(seq int, submit bool) error {
	return s.withFile(seq, func(f *fileState) error {
		res := FileResult{File: f.File, Status: markdown.StatusCancelled}
		if submit {
			res.Review = f.buildReview()
			res.Status = res.Review.Status()
		}
		s.results = append(s.results, res)
		s.advance()
		return nil
	})
}

func (s *session) phaseError() error {
	if s.phase == phaseDone {
		return errFinished
	}
	return fmt.Errorf("%w (%s)", errPhase, s.phase)
}

// newID returns a random 16-hex-character identifier.
func newID() string {
	var b [8]byte
	_, _ = rand.Read(b[:]) // crypto/rand.Read never returns an error
	return hex.EncodeToString(b[:])
}

// openAsset opens a file relative to the directory of the file under review.
// os.Root keeps the lookup inside that directory, including through
// symlinks and "..".
func (s *session) openAsset(seq int, name string) (*os.File, error) {
	var dir string
	err := s.withFile(seq, func(f *fileState) error {
		dir = filepath.Dir(f.Path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, errNotFound)
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, errNotFound)
	}
	return f, nil
}
