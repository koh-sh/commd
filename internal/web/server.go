// Package web serves the review UI in a web browser. It is the browser
// counterpart of the tui package: the same documents and comment model, with
// the review result handed back to the caller when the reviewer submits or
// quits in the page.
package web

import (
	"cmp"
	"context"
	"crypto/subtle"
	"embed"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/koh-sh/commd/internal/markdown"
)

//go:embed static
var staticFiles embed.FS

// tokenHeader carries the session token on API requests. Requiring a custom
// header also forces a CORS preflight for cross-origin requests, which this
// server never approves.
const tokenHeader = "X-Commd-Token"

// shutdownTimeout bounds how long in-flight requests may take after the
// review ends.
const shutdownTimeout = 5 * time.Second

// maxRequestBody caps API request bodies.
const maxRequestBody = 1 << 20

// Options configures Serve.
type Options struct {
	Port  int    // TCP port on 127.0.0.1; 0 picks a free port
	Theme string // initial color theme: "dark" or "light"
	// Open opens the URL in a browser. nil only prints it.
	Open func(url string) error
	Log  io.Writer // receives the URL and warnings; nil discards them
}

// Serve starts the review server on the loopback interface and blocks until
// every file was finished or skipped in the browser (or the picker was
// cancelled), or ctx is cancelled, which abandons the whole session. It
// returns one result per file that was loaded, in review order: none when
// the picker was cancelled or the session was interrupted.
func Serve(ctx context.Context, review Review, opts Options) ([]markdown.FileResult, error) {
	log := cmp.Or[io.Writer](opts.Log, io.Discard)
	token := newID() + newID()
	s := newSession(review, token, opts)
	if s.phase == phaseDone {
		return <-s.done, nil // nothing could be loaded; the skips were logged
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(opts.Port)))
	if err != nil {
		return nil, fmt.Errorf("starting web server: %w", err)
	}
	srv := &http.Server{
		Handler:           newHandler(s),
		ReadHeaderTimeout: 10 * time.Second,
		// API requests and responses are small; images are local files.
		ReadTimeout:  30 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  2 * time.Minute,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	// The token travels in the fragment so it never appears in request lines
	// or Referer headers.
	url := fmt.Sprintf("http://%s/#token=%s", ln.Addr(), token)
	fmt.Fprintf(log, "Reviewing in the browser: %s\n", url)
	if opts.Open != nil {
		if err := opts.Open(url); err != nil {
			fmt.Fprintf(log, "Could not open a browser (%v); open the URL above.\n", err)
		}
	}

	var res []markdown.FileResult
	select {
	case res = <-s.done:
	case <-ctx.Done():
	case err := <-serveErr:
		return nil, fmt.Errorf("web server: %w", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	// Shutdown waits for the last finish request's response to be written.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(log, "commd: warning: stopping web server: %v\n", err)
	}
	return res, nil
}

// newHandler returns the HTTP handler: the embedded page plus the JSON API.
// Every API call returns the whole session state, which the page renders.
func newHandler(s *session) http.Handler {
	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err) // the embedded directory always exists
	}
	api := &apiHandler{s: s}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/state", api.getState)
	mux.HandleFunc("POST /api/reload", api.reload)
	mux.HandleFunc("GET /api/files/{seq}/search", api.search)
	mux.HandleFunc("POST /api/pick", api.pick)
	mux.HandleFunc("POST /api/files/{seq}/comments", api.addComment)
	mux.HandleFunc("PATCH /api/files/{seq}/comments/{id}", api.updateComment)
	mux.HandleFunc("DELETE /api/files/{seq}/comments/{id}", api.deleteComment)
	mux.HandleFunc("PUT /api/files/{seq}/viewed/{section}", api.setViewed)
	mux.HandleFunc("POST /api/files/{seq}/finish", api.finish)
	mux.HandleFunc("GET /assets/{seq}/{path...}", api.serveAsset)
	return securityHeaders(requireToken(s.token, mux))
}

// securityHeaders restricts the page to its own scripts and styles. Images
// may come from anywhere since reviewed documents link to them.
func securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		h := w.Header()
		h.Set("Content-Security-Policy", "default-src 'self'; img-src * data:; style-src 'self' 'unsafe-inline'; frame-ancestors 'none'")
		h.Set("Referrer-Policy", "no-referrer")
		h.Set("X-Content-Type-Options", "nosniff")
		h.Set("Cache-Control", "no-store")
		next.ServeHTTP(w, r)
	})
}

// requireToken rejects API requests without the session token. Static files
// carry no review data and are served without it.
func requireToken(token string, next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if strings.HasPrefix(r.URL.Path, "/api/") && !validToken(r.Header.Get(tokenHeader), token) {
			writeError(w, http.StatusUnauthorized, errors.New("invalid session token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

// validToken compares tokens in constant time.
func validToken(got, want string) bool {
	return subtle.ConstantTimeCompare([]byte(got), []byte(want)) == 1
}

type apiHandler struct {
	s *session
}

func (a *apiHandler) getState(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.s.state())
}

// reload reads the file under review again and returns the state with the
// outcome. The page calls it when it loads and on R.
func (a *apiHandler) reload(w http.ResponseWriter, _ *http.Request) {
	res, ok := a.s.reload()
	state := a.s.state()
	if ok {
		state.Reload = &reloadJSON{Message: res.Message(), Changed: res.Changed, Failed: res.Err != nil, Sections: res.Sections}
	}
	writeJSON(w, http.StatusOK, state)
}

// search returns the IDs of the sections a search for the q parameter shows
// in the section list. It changes nothing, so it returns only the IDs.
func (a *apiHandler) search(w http.ResponseWriter, r *http.Request) {
	var ids []string
	err := a.s.withFile(fileSeq(r), func(f *fileState) error {
		ids = f.search(r.URL.Query().Get("q"))
		return nil
	})
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string][]string{"sections": ids})
}

func (a *apiHandler) pick(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Paths []string `json:"paths"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.respond(w, a.s.pick(in.Paths))
}

func (a *apiHandler) addComment(w http.ResponseWriter, r *http.Request) {
	var in markdown.ReviewComment
	if !decode(w, r, &in) {
		return
	}
	a.respond(w, a.s.withFile(fileSeq(r), func(f *fileState) error {
		return f.addComment(in)
	}))
}

func (a *apiHandler) updateComment(w http.ResponseWriter, r *http.Request) {
	var in markdown.ReviewComment
	if !decode(w, r, &in) {
		return
	}
	a.respond(w, a.s.withFile(fileSeq(r), func(f *fileState) error {
		return f.updateComment(r.PathValue("id"), in)
	}))
}

func (a *apiHandler) deleteComment(w http.ResponseWriter, r *http.Request) {
	a.respond(w, a.s.withFile(fileSeq(r), func(f *fileState) error {
		return f.DeleteComment(r.PathValue("id"))
	}))
}

func (a *apiHandler) setViewed(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Viewed bool `json:"viewed"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.respond(w, a.s.withFile(fileSeq(r), func(f *fileState) error {
		return f.SetViewed(r.PathValue("section"), in.Viewed)
	}))
}

func (a *apiHandler) finish(w http.ResponseWriter, r *http.Request) {
	var in struct {
		Action string `json:"action"` // "submit" or "quit"
	}
	if !decode(w, r, &in) {
		return
	}
	if in.Action != "submit" && in.Action != "quit" {
		writeError(w, http.StatusBadRequest, fmt.Errorf("unknown action %q", in.Action))
		return
	}
	a.respond(w, a.s.finish(fileSeq(r), in.Action == "submit"))
}

// respond writes the session state after a successful change, or the error.
func (a *apiHandler) respond(w http.ResponseWriter, err error) {
	if err != nil {
		writeSessionError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, a.s.state())
}

// fileSeq returns the {seq} path value; an unparsable value never matches.
func fileSeq(r *http.Request) int {
	seq, err := strconv.Atoi(r.PathValue("seq"))
	if err != nil {
		return -1
	}
	return seq
}

func decode(w http.ResponseWriter, r *http.Request, v any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, maxRequestBody)).Decode(v); err != nil {
		writeError(w, http.StatusBadRequest, fmt.Errorf("invalid request body: %w", err))
		return false
	}
	return true
}

// writeSessionError maps session errors to HTTP status codes. Conflicts
// (the session moved on) tell the page to reload the state.
func writeSessionError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, markdown.ErrNotFound):
		writeError(w, http.StatusNotFound, err)
	case errors.Is(err, errFinished), errors.Is(err, errStale), errors.Is(err, errPhase):
		writeError(w, http.StatusConflict, err)
	default:
		writeError(w, http.StatusBadRequest, err)
	}
}

func writeError(w http.ResponseWriter, status int, err error) {
	writeJSON(w, status, map[string]string{"error": err.Error()})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v) // the client is gone if this fails
}
