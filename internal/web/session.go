package web

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
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
	Load func(path string) (f markdown.File, ok bool)
}

// Session phases, as reported to the browser.
const (
	phasePick   = "pick"
	phaseReview = "review"
	phaseDone   = "done"
)

// errFinished is returned for any change after the session ended.
var errFinished = errors.New("review already finished")

// errStale rejects a request for a file that is no longer under review, e.g.
// from a second tab that missed a finish.
var errStale = errors.New("that file is no longer under review; reload the page")

// errPhase rejects a request that does not fit the current phase, e.g. a
// comment while the picker is shown.
var errPhase = errors.New("not possible in the current phase")

// session holds the state of a browser review. All methods are safe for
// concurrent use by HTTP handlers.
type session struct {
	mu        sync.Mutex
	review    Review
	phase     string
	multiFile bool     // several files were chosen: dialogs say finish/skip this file
	queue     []string // files still to review after the current one
	current   *fileState
	seq       int // increments per reviewed file; requests carry it to detect stale pages
	results   []markdown.FileResult
	// done receives the results once when the session ends: one per file
	// that was loaded, in review order.
	done  chan []markdown.FileResult
	token string // authenticates the API and the /assets/ URLs
	theme string // initial color theme of the page
}

func newSession(review Review, token, theme string) *session {
	s := &session{review: review, done: make(chan []markdown.FileResult, 1), token: token, theme: theme}
	if len(review.Pick) > 0 {
		s.phase = phasePick
	} else {
		s.start(review.Paths)
	}
	return s
}

// start begins reviewing paths in order.
func (s *session) start(paths []string) {
	s.multiFile = len(paths) > 1
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
			s.seq++
			s.current = newFileState(f, assetURL(s.seq, s.token))
			s.phase = phaseReview
			return
		}
	}
	s.phase = phaseDone
	s.done <- s.results
}

// pick starts the review of the chosen files, keeping the picker's order.
// Choosing nothing (which is how the picker is cancelled) ends the session
// without reviews.
func (s *session) pick(paths []string) error {
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
		res := markdown.FileResult{File: f.File, Status: markdown.StatusCancelled}
		if submit {
			res.Review = f.Result()
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
