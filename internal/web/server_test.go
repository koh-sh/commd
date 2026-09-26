package web

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
	"time"

	"github.com/koh-sh/commd/internal/markdown"
)

const testToken = "secret"

// do sends a request to h and returns the status and body.
func do(t *testing.T, h http.Handler, method, path, token, body string) (int, string) {
	t.Helper()
	req := httptest.NewRequest(method, path, strings.NewReader(body))
	if token != "" {
		req.Header.Set(tokenHeader, token)
	}
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)
	return rec.Code, rec.Body.String()
}

func singleFileSession(t *testing.T) *session {
	t.Helper()
	return newSession(Review{Paths: []string{"doc.md"}, Load: func(string) (File, bool) { return testFile(t, false), true }})
}

func TestHandlerToken(t *testing.T) {
	h := newHandler(singleFileSession(t), testToken, "dark")
	tests := []struct {
		name   string
		path   string
		token  string
		status int
	}{
		{name: "page without token", path: "/", status: http.StatusOK},
		{name: "script without token", path: "/main.js", status: http.StatusOK},
		{name: "api without token", path: "/api/state", status: http.StatusUnauthorized},
		{name: "api with wrong token", path: "/api/state", token: "guess", status: http.StatusUnauthorized},
		{name: "api with token", path: "/api/state", token: testToken, status: http.StatusOK},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if status, body := do(t, h, http.MethodGet, tt.path, tt.token, ""); status != tt.status {
				t.Errorf("status = %d, want %d (body %s)", status, tt.status, body)
			}
		})
	}
}

func TestHandlerAPI(t *testing.T) {
	s := singleFileSession(t)
	h := newHandler(s, testToken, "light")

	// Create a comment first so later steps can address it by ID.
	status, body := do(t, h, http.MethodPost, "/api/files/1/comments", testToken,
		`{"sectionId":"S1","action":"issue","decoration":"","body":"first"}`)
	if status != http.StatusOK {
		t.Fatalf("add: status %d, body %s", status, body)
	}
	var state stateJSON
	if err := json.Unmarshal([]byte(body), &state); err != nil {
		t.Fatal(err)
	}
	commentPath := "/api/files/1/comments/" + state.File.Comments[0].ID

	tests := []struct {
		name     string
		method   string
		path     string
		body     string
		status   int
		wantBody string // substring of the response body
	}{
		{name: "state", method: http.MethodGet, path: "/api/state", status: http.StatusOK, wantBody: `"theme":"light"`},
		{name: "invalid json", method: http.MethodPost, path: "/api/files/1/comments", body: "{", status: http.StatusBadRequest},
		{name: "invalid comment", method: http.MethodPost, path: "/api/files/1/comments", body: `{"sectionId":"S1","action":"note","body":""}`, status: http.StatusBadRequest, wantBody: "empty"},
		{name: "stale file", method: http.MethodPost, path: "/api/files/7/comments", body: `{"sectionId":"S1","action":"note","body":"b"}`, status: http.StatusConflict},
		{name: "non-numeric file", method: http.MethodPost, path: "/api/files/x/comments", body: `{"sectionId":"S1","action":"note","body":"b"}`, status: http.StatusConflict},
		{name: "pick while reviewing", method: http.MethodPost, path: "/api/pick", body: `{"paths":["doc.md"]}`, status: http.StatusConflict},
		{name: "update", method: http.MethodPatch, path: commentPath, body: `{"action":"praise","decoration":"","body":"nice"}`, status: http.StatusOK, wantBody: `"body":"nice"`},
		{name: "set viewed", method: http.MethodPut, path: "/api/files/1/viewed/S1", body: `{"viewed":true}`, status: http.StatusOK, wantBody: `"viewed":["S1"]`},
		{name: "delete", method: http.MethodDelete, path: commentPath, status: http.StatusOK, wantBody: `"comments":[]`},
		{name: "delete again", method: http.MethodDelete, path: commentPath, status: http.StatusNotFound},
		{name: "unknown finish action", method: http.MethodPost, path: "/api/files/1/finish", body: `{"action":"maybe"}`, status: http.StatusBadRequest},
		{name: "finish", method: http.MethodPost, path: "/api/files/1/finish", body: `{"action":"submit"}`, status: http.StatusOK, wantBody: `"phase":"done"`},
		{name: "change after the end", method: http.MethodPut, path: "/api/files/1/viewed/S1", body: `{"viewed":false}`, status: http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			status, body := do(t, h, tt.method, tt.path, testToken, tt.body)
			if status != tt.status {
				t.Errorf("status = %d, want %d (body %s)", status, tt.status, body)
			}
			if !strings.Contains(body, tt.wantBody) {
				t.Errorf("body = %s, want it to contain %s", body, tt.wantBody)
			}
		})
	}
}

func TestHandlerPick(t *testing.T) {
	loader := &testLoader{t: t, known: []string{"a.md", "b.md"}}
	s := newSession(Review{Pick: []string{"a.md", "b.md"}, Load: loader.load})
	h := newHandler(s, testToken, "dark")

	status, body := do(t, h, http.MethodGet, "/api/state", testToken, "")
	if status != http.StatusOK || !strings.Contains(body, `"phase":"pick"`) || !strings.Contains(body, `"pick":["a.md","b.md"]`) {
		t.Fatalf("state: %d %s", status, body)
	}
	status, body = do(t, h, http.MethodPost, "/api/pick", testToken, `{"paths":["b.md"]}`)
	if status != http.StatusOK || !strings.Contains(body, `"phase":"review"`) || !strings.Contains(body, `"path":"b.md"`) {
		t.Fatalf("pick: %d %s", status, body)
	}
	if !strings.Contains(body, `"multi":false`) {
		t.Errorf("one picked file should not be a multi-file review: %s", body)
	}
}

func TestStateEscapesRawHTML(t *testing.T) {
	doc := mustParse(t, "## A\n\n<script>alert(1)</script>\n\n[x](javascript:alert(1))\n")
	html := newFileState(File{Path: "a.md", Doc: doc}).sectionsJSON(nil)[0].HTML
	for _, bad := range []string{"<script>", "javascript:"} {
		if strings.Contains(html, bad) {
			t.Errorf("rendered HTML contains %q:\n%s", bad, html)
		}
	}
}

func TestStateSectionHTML(t *testing.T) {
	doc := mustParse(t, "Title\n=====\n\nIntro.\n\n## Use `commd`\n\nBody.\n\n### Child\n\nChild body.\n")
	sections := newFileState(File{Path: "a.md", Doc: doc}).sectionsJSON(nil)
	tests := []struct {
		id      string
		want    []string
		notWant []string
	}{
		{id: "overview", want: []string{"<h1>Title</h1>", "<p>Intro.</p>"}, notWant: []string{"Use"}},
		{id: "S1", want: []string{"<h2>Use <code>commd</code></h2>", "<p>Body.</p>"}, notWant: []string{"Child"}},
		{id: "S1.1", want: []string{"<h3>Child</h3>", "<p>Child body.</p>"}},
	}
	for _, tt := range tests {
		t.Run(tt.id, func(t *testing.T) {
			var html string
			for _, s := range sections {
				if s.ID == tt.id {
					html = s.HTML
				}
			}
			for _, w := range tt.want {
				if !strings.Contains(html, w) {
					t.Errorf("HTML missing %q:\n%s", w, html)
				}
			}
			for _, w := range tt.notWant {
				if strings.Contains(html, w) {
					t.Errorf("HTML unexpectedly contains %q:\n%s", w, html)
				}
			}
		})
	}
}

func TestServe(t *testing.T) {
	urlPattern := regexp.MustCompile(`(http://\S+)/#token=(\w+)`)
	tests := []struct {
		name       string
		finish     string // action posted for the file; "" cancels ctx instead
		wantStatus []markdown.Status
	}{
		{name: "submit from the browser", finish: "submit", wantStatus: []markdown.Status{markdown.StatusApproved}},
		{name: "quit from the browser", finish: "quit", wantStatus: []markdown.Status{markdown.StatusCancelled}},
		{name: "interrupted", wantStatus: nil},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			logR, logW := io.Pipe()
			opened := make(chan string, 1)
			type served struct {
				res Result
				err error
			}
			done := make(chan served, 1)
			go func() {
				review := Review{Paths: []string{"doc.md"}, Load: func(string) (File, bool) { return testFile(t, false), true }}
				res, err := Serve(ctx, review, Options{
					Log:  logW,
					Open: func(url string) error { opened <- url; return nil },
				})
				done <- served{res, err}
			}()

			line, err := readLine(logR)
			if err != nil {
				t.Fatal(err)
			}
			go func() { _, _ = io.Copy(io.Discard, logR) }()
			m := urlPattern.FindStringSubmatch(line)
			if m == nil {
				t.Fatalf("no URL in log line %q", line)
			}
			if got := <-opened; got != m[0] {
				t.Errorf("opened %q, want %q", got, m[0])
			}

			if tt.finish == "" {
				cancel()
			} else {
				req, _ := http.NewRequest(http.MethodPost, m[1]+"/api/files/1/finish", strings.NewReader(`{"action":"`+tt.finish+`"}`))
				req.Header.Set(tokenHeader, m[2])
				resp, err := http.DefaultClient.Do(req)
				if err != nil {
					t.Fatal(err)
				}
				resp.Body.Close()
				if resp.StatusCode != http.StatusOK {
					t.Fatalf("finish status = %d", resp.StatusCode)
				}
			}

			select {
			case got := <-done:
				if got.err != nil {
					t.Fatal(got.err)
				}
				var statuses []markdown.Status
				for _, f := range got.res.Files {
					statuses = append(statuses, f.Status)
				}
				if len(statuses) != len(tt.wantStatus) || (len(statuses) > 0 && statuses[0] != tt.wantStatus[0]) {
					t.Errorf("statuses = %v, want %v", statuses, tt.wantStatus)
				}
			case <-time.After(10 * time.Second):
				t.Fatal("Serve did not return")
			}
		})
	}
}

func TestServeNothingToReview(t *testing.T) {
	var log bytes.Buffer
	review := Review{Paths: []string{"gone.md"}, Load: func(string) (File, bool) { return File{}, false }}
	res, err := Serve(context.Background(), review, Options{Log: &log, Open: func(string) error {
		t.Error("no browser should open when nothing can be reviewed")
		return nil
	}})
	if err != nil {
		t.Fatal(err)
	}
	if len(res.Files) != 0 || log.Len() != 0 {
		t.Errorf("result = %+v, log = %q; want nothing", res, log.String())
	}
}

// readLine reads up to the first newline.
func readLine(r io.Reader) (string, error) {
	var buf bytes.Buffer
	b := make([]byte, 1)
	for {
		if _, err := r.Read(b); err != nil {
			return buf.String(), err
		}
		if b[0] == '\n' {
			return buf.String(), nil
		}
		buf.WriteByte(b[0])
	}
}

func TestStateEmptyDocument(t *testing.T) {
	f := newFileState(File{Path: "empty.md", Doc: mustParse(t, "")})
	body, err := json.Marshal(f.toJSON(nil))
	if err != nil {
		t.Fatal(err)
	}
	// The page reads these as arrays; null would break it.
	for _, want := range []string{`"sections":[]`, `"comments":[]`, `"viewed":[]`} {
		if !strings.Contains(string(body), want) {
			t.Errorf("state %s lacks %s", body, want)
		}
	}
}

func TestAssetURL(t *testing.T) {
	rewrite := assetURL(3, "tok")
	tests := []struct {
		dest string
		want string
	}{
		{dest: "vhs/demo.gif", want: "/assets/3/vhs/demo.gif?t=tok"},
		{dest: "./img/a%20b.png", want: "/assets/3/./img/a%20b.png?t=tok"},
		{dest: "../up.png", want: "/assets/3/../up.png?t=tok"}, // refused when served
		{dest: "https://example.com/a.png", want: "https://example.com/a.png"},
		{dest: "/abs/a.png", want: "/abs/a.png"},
		{dest: "data:image/png;base64,AAAA", want: "data:image/png;base64,AAAA"},
		{dest: "#anchor", want: "#anchor"},
	}
	for _, tt := range tests {
		t.Run(tt.dest, func(t *testing.T) {
			if got := rewrite(tt.dest); got != tt.want {
				t.Errorf("assetURL(%q) = %q, want %q", tt.dest, got, tt.want)
			}
		})
	}
}

func TestServeAsset(t *testing.T) {
	dir := t.TempDir()
	for name, content := range map[string]string{
		"doc.md":         "![demo](img/demo.gif)\n",
		"img/demo.gif":   "GIF89a",
		"img/pic.svg":    "<svg/>",
		"notes.txt":      "secret",
		"../outside.png": "x",
		"img/sub/.keep":  "",
		"img/dir.png/.x": "",
	} {
		p := filepath.Join(dir, "docs", name)
		if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(p, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	docPath := filepath.Join(dir, "docs", "doc.md")
	s := newSession(Review{Paths: []string{docPath}, Load: func(string) (File, bool) {
		return File{Path: docPath, Doc: mustParse(t, "![demo](img/demo.gif)\n")}, true
	}})
	h := newHandler(s, testToken, "dark")

	// The rendered document points at the asset route.
	_, body := do(t, h, http.MethodGet, "/api/state", testToken, "")
	if !strings.Contains(body, `src=\"/assets/1/img/demo.gif?t=secret\"`) {
		t.Errorf("state does not rewrite the image: %s", body)
	}

	tests := []struct {
		name     string
		path     string
		status   int
		wantType string
	}{
		{name: "image", path: "/assets/1/img/demo.gif?t=" + testToken, status: http.StatusOK, wantType: "image/gif"},
		{name: "svg", path: "/assets/1/img/pic.svg?t=" + testToken, status: http.StatusOK, wantType: "image/svg+xml"},
		{name: "no token", path: "/assets/1/img/demo.gif", status: http.StatusUnauthorized},
		{name: "not an image", path: "/assets/1/notes.txt?t=" + testToken, status: http.StatusNotFound},
		{name: "outside the directory", path: "/assets/1/..%2Foutside.png?t=" + testToken, status: http.StatusNotFound},
		{name: "directory", path: "/assets/1/img/dir.png?t=" + testToken, status: http.StatusNotFound},
		{name: "missing", path: "/assets/1/img/none.png?t=" + testToken, status: http.StatusNotFound},
		{name: "stale file", path: "/assets/2/img/demo.gif?t=" + testToken, status: http.StatusConflict},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			req := httptest.NewRequest(http.MethodGet, tt.path, nil)
			rec := httptest.NewRecorder()
			h.ServeHTTP(rec, req)
			if rec.Code != tt.status {
				t.Fatalf("status = %d, want %d (%s)", rec.Code, tt.status, rec.Body.String())
			}
			if tt.wantType != "" {
				if got := rec.Header().Get("Content-Type"); got != tt.wantType {
					t.Errorf("Content-Type = %q, want %q", got, tt.wantType)
				}
				if csp := rec.Header().Get("Content-Security-Policy"); !strings.Contains(csp, "sandbox") {
					t.Errorf("CSP %q lacks sandbox", csp)
				}
			}
		})
	}
}
