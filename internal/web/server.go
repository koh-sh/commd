// Package web serves the review UI in a web browser. It is the browser
// counterpart of the tui package: the same documents and comment model, with
// the review result handed back to the caller when the reviewer submits or
// quits in the page.
package web

import (
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
	"path"
	"strconv"
	"strings"
	"time"
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
	Log  io.Writer // receives the URL and warnings
}

// Serve starts the review server on the loopback interface and blocks until
// every file was finished or skipped in the browser (or the picker was
// cancelled), or ctx is cancelled, which abandons the whole session.
func Serve(ctx context.Context, review Review, opts Options) (Result, error) {
	s := newSession(review)
	if s.phase == phaseDone {
		return <-s.done, nil // nothing could be loaded; Load reported why
	}
	ln, err := net.Listen("tcp", net.JoinHostPort("127.0.0.1", strconv.Itoa(opts.Port)))
	if err != nil {
		return Result{}, fmt.Errorf("starting web server: %w", err)
	}
	token := newID() + newID()
	srv := &http.Server{
		Handler:           newHandler(s, token, opts.Theme),
		ReadHeaderTimeout: 10 * time.Second,
	}
	serveErr := make(chan error, 1)
	go func() { serveErr <- srv.Serve(ln) }()

	// The token travels in the fragment so it never appears in request lines
	// or Referer headers.
	url := fmt.Sprintf("http://%s/#token=%s", ln.Addr(), token)
	fmt.Fprintf(opts.Log, "Reviewing in the browser: %s\n", url)
	if opts.Open != nil {
		if err := opts.Open(url); err != nil {
			fmt.Fprintf(opts.Log, "Could not open a browser (%v); open the URL above.\n", err)
		}
	}

	var res Result
	select {
	case res = <-s.done:
	case <-ctx.Done():
	case err := <-serveErr:
		return Result{}, fmt.Errorf("web server: %w", err)
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), shutdownTimeout)
	defer cancel()
	// Shutdown waits for the last finish request's response to be written.
	if err := srv.Shutdown(shutdownCtx); err != nil {
		fmt.Fprintf(opts.Log, "commd: warning: stopping web server: %v\n", err)
	}
	return res, nil
}

// newHandler returns the HTTP handler: the embedded page plus the JSON API.
// Every API call returns the whole session state, which the page renders.
func newHandler(s *session, token, theme string) http.Handler {
	static, err := fs.Sub(staticFiles, "static")
	if err != nil {
		panic(err) // the embedded directory always exists
	}
	api := &apiHandler{s: s, token: token, theme: theme}
	mux := http.NewServeMux()
	mux.Handle("GET /", http.FileServerFS(static))
	mux.HandleFunc("GET /api/state", api.getState)
	mux.HandleFunc("GET /api/files/{seq}/search", api.search)
	mux.HandleFunc("POST /api/pick", api.pick)
	mux.HandleFunc("POST /api/files/{seq}/comments", api.addComment)
	mux.HandleFunc("PATCH /api/files/{seq}/comments/{id}", api.updateComment)
	mux.HandleFunc("DELETE /api/files/{seq}/comments/{id}", api.deleteComment)
	mux.HandleFunc("PUT /api/files/{seq}/viewed/{section}", api.setViewed)
	mux.HandleFunc("POST /api/files/{seq}/finish", api.finish)
	mux.HandleFunc("GET /assets/{seq}/{path...}", api.serveAsset)
	return securityHeaders(requireToken(token, mux))
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
		if strings.HasPrefix(r.URL.Path, "/api/") &&
			subtle.ConstantTimeCompare([]byte(r.Header.Get(tokenHeader)), []byte(token)) != 1 {
			writeError(w, http.StatusUnauthorized, errors.New("invalid session token"))
			return
		}
		next.ServeHTTP(w, r)
	})
}

type apiHandler struct {
	s     *session
	token string
	theme string
}

func (a *apiHandler) getState(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, a.s.state(a.theme, a.token))
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
		Paths  []string `json:"paths"`
		Cancel bool     `json:"cancel"`
	}
	if !decode(w, r, &in) {
		return
	}
	a.respond(w, a.s.pick(in.Paths, in.Cancel))
}

func (a *apiHandler) addComment(w http.ResponseWriter, r *http.Request) {
	var in commentInput
	if !decode(w, r, &in) {
		return
	}
	a.respond(w, a.s.withFile(fileSeq(r), func(f *fileState) error {
		_, err := f.addComment(in)
		return err
	}))
}

func (a *apiHandler) updateComment(w http.ResponseWriter, r *http.Request) {
	var in commentInput
	if !decode(w, r, &in) {
		return
	}
	a.respond(w, a.s.withFile(fileSeq(r), func(f *fileState) error {
		_, err := f.updateComment(r.PathValue("id"), in)
		return err
	}))
}

func (a *apiHandler) deleteComment(w http.ResponseWriter, r *http.Request) {
	a.respond(w, a.s.withFile(fileSeq(r), func(f *fileState) error {
		return f.deleteComment(r.PathValue("id"))
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
		return f.setViewed(r.PathValue("section"), in.Viewed)
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
	writeJSON(w, http.StatusOK, a.s.state(a.theme, a.token))
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
	case errors.Is(err, errNotFound):
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

// assetTypes are the files /assets/ serves: images referenced by the
// reviewed document.
var assetTypes = map[string]string{
	".png":  "image/png",
	".jpg":  "image/jpeg",
	".jpeg": "image/jpeg",
	".gif":  "image/gif",
	".webp": "image/webp",
	".avif": "image/avif",
	".svg":  "image/svg+xml",
	".bmp":  "image/bmp",
	".ico":  "image/x-icon",
}

// serveAsset serves an image referenced by a relative path in the document
// under review, from the document's directory (never outside it). Images
// cannot send the token header, so it comes in the t query parameter.
func (a *apiHandler) serveAsset(w http.ResponseWriter, r *http.Request) {
	if subtle.ConstantTimeCompare([]byte(r.URL.Query().Get("t")), []byte(a.token)) != 1 {
		writeError(w, http.StatusUnauthorized, errors.New("invalid session token"))
		return
	}
	name := r.PathValue("path")
	contentType, ok := assetTypes[strings.ToLower(path.Ext(name))]
	if !ok {
		writeError(w, http.StatusNotFound, fmt.Errorf("%s: not an image", name))
		return
	}
	f, err := a.s.openAsset(fileSeq(r), name)
	if err != nil {
		writeSessionError(w, err)
		return
	}
	defer f.Close()
	st, err := f.Stat()
	if err != nil || st.IsDir() {
		writeError(w, http.StatusNotFound, fmt.Errorf("%s: %w", name, errNotFound))
		return
	}
	w.Header().Set("Content-Type", contentType)
	// An SVG opened on its own must not run scripts with this origin.
	w.Header().Set("Content-Security-Policy", "default-src 'none'; style-src 'unsafe-inline'; img-src data:; sandbox")
	http.ServeContent(w, r, name, st.ModTime(), f)
}
