package github

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func writeJSON(t *testing.T, w http.ResponseWriter, v any) {
	t.Helper()
	w.Header().Set("Content-Type", "application/json")
	if err := json.NewEncoder(w).Encode(v); err != nil {
		t.Fatalf("failed to encode JSON: %v", err)
	}
}

func TestListMDFiles(t *testing.T) {
	tests := []struct {
		name      string
		files     []map[string]string
		wantPaths []string
		wantErr   bool
	}{
		{
			name: "filters md files and excludes deleted",
			files: []map[string]string{
				{"filename": "README.md", "status": "modified"},
				{"filename": "docs/guide.md", "status": "added"},
				{"filename": "main.go", "status": "modified"},
				{"filename": "old.md", "status": "removed"},
			},
			wantPaths: []string{"README.md", "docs/guide.md"},
		},
		{
			name:      "no md files returns empty",
			files:     []map[string]string{{"filename": "main.go", "status": "modified"}},
			wantPaths: nil,
		},
		{
			name: "renamed md file included",
			files: []map[string]string{
				{"filename": "new-name.md", "status": "renamed"},
			},
			wantPaths: []string{"new-name.md"},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /repos/owner/repo/pulls/1/files", func(w http.ResponseWriter, _ *http.Request) {
				writeJSON(t, w, tt.files)
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			client := NewClientWithHTTP(srv.Client(), srv.URL+"/")
			ref := &PRRef{Owner: "owner", Repo: "repo", Number: 1}

			got, err := client.ListMDFiles(context.Background(), ref)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if len(got) != len(tt.wantPaths) {
				t.Fatalf("got %d files, want %d", len(got), len(tt.wantPaths))
			}
			for i, want := range tt.wantPaths {
				if got[i].Path != want {
					t.Errorf("file[%d].Path = %q, want %q", i, got[i].Path, want)
				}
			}
		})
	}
}

func TestFetchFileContent(t *testing.T) {
	tests := []struct {
		name        string
		path        string
		fileContent string
		statusCode  int
		oversized   bool
		wantErr     bool
	}{
		{
			name:        "fetches file content",
			path:        "README.md",
			fileContent: "# Hello World\n\nThis is a test.",
		},
		{
			name:        "fetches oversized file via download URL",
			path:        "BIG.md",
			fileContent: "# Big file\n\ncontent over 1 MB",
			oversized:   true,
		},
		{
			name:       "file not found",
			path:       "missing.md",
			statusCode: 404,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			var srv *httptest.Server

			// Contents endpoint
			mux.HandleFunc("GET /repos/owner/repo/contents/", func(w http.ResponseWriter, r *http.Request) {
				if tt.statusCode == 404 {
					http.NotFound(w, r)
					return
				}
				if tt.oversized {
					// Files between 1 MB and 100 MB return an empty body with
					// encoding "none" plus a download URL for the raw content.
					writeJSON(t, w, map[string]any{
						"type":         "file",
						"encoding":     "none",
						"content":      "",
						"download_url": srv.URL + "/raw/" + tt.path,
					})
					return
				}
				encoded := base64.StdEncoding.EncodeToString([]byte(tt.fileContent))
				writeJSON(t, w, map[string]any{
					"type":     "file",
					"encoding": "base64",
					"content":  encoded,
				})
			})

			// Raw download endpoint serving full file bytes for oversized files.
			mux.HandleFunc("GET /raw/", func(w http.ResponseWriter, _ *http.Request) {
				_, _ = w.Write([]byte(tt.fileContent))
			})

			srv = httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			client := NewClientWithHTTP(srv.Client(), srv.URL+"/")
			ref := &PRRef{Owner: "owner", Repo: "repo", Number: 1}

			got, err := client.FetchFileContent(context.Background(), ref, tt.path, "abc123")
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}

			if string(got) != tt.fileContent {
				t.Errorf("got %q, want %q", string(got), tt.fileContent)
			}
		})
	}
}

func TestGetHeadSHA(t *testing.T) {
	tests := []struct {
		name       string
		sha        string
		ref        string
		statusCode int
		wantSHA    string
		wantErr    bool
	}{
		{
			name:    "returns SHA",
			sha:     "abc123",
			ref:     "feature",
			wantSHA: "abc123",
		},
		{
			name:    "falls back to ref when SHA is empty",
			sha:     "",
			ref:     "feature-branch",
			wantSHA: "feature-branch",
		},
		{
			name:       "PR not found",
			statusCode: 404,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("GET /repos/owner/repo/pulls/1", func(w http.ResponseWriter, r *http.Request) {
				if tt.statusCode == 404 {
					http.NotFound(w, r)
					return
				}
				writeJSON(t, w, map[string]any{
					"head": map[string]any{"sha": tt.sha, "ref": tt.ref},
				})
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			client := NewClientWithHTTP(srv.Client(), srv.URL+"/")
			ref := &PRRef{Owner: "owner", Repo: "repo", Number: 1}

			got, err := client.GetHeadSHA(context.Background(), ref)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.wantSHA {
				t.Errorf("got %q, want %q", got, tt.wantSHA)
			}
		})
	}
}

func TestResolveToken(t *testing.T) {
	tests := []struct {
		name    string
		envVar  string
		wantErr bool
	}{
		{
			name:   "from GITHUB_TOKEN env var",
			envVar: "test-token-123",
		},
		{
			name:    "no token available",
			envVar:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envVar != "" {
				t.Setenv("GITHUB_TOKEN", tt.envVar)
			} else {
				t.Setenv("GITHUB_TOKEN", "")
				t.Setenv("PATH", "")
			}

			got, err := resolveToken()
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.envVar {
				t.Errorf("got %q, want %q", got, tt.envVar)
			}
		})
	}
}

func TestNewClient(t *testing.T) {
	t.Setenv("GITHUB_TOKEN", "test-token")
	client, err := NewClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if client == nil {
		t.Fatal("expected non-nil client")
	}
}

func TestNewClientWithCustomBaseURL(t *testing.T) {
	// Start a test server to verify requests are directed to the custom URL.
	mux := http.NewServeMux()
	mux.HandleFunc("GET /repos/owner/repo/pulls/1", func(w http.ResponseWriter, _ *http.Request) {
		writeJSON(t, w, map[string]any{
			"head": map[string]any{"sha": "abc123", "ref": "feature"},
		})
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	t.Setenv("GITHUB_TOKEN", "test-token")
	t.Setenv("COMMD_GITHUB_API_URL", srv.URL+"/")

	client, err := NewClient()
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Verify the client actually talks to the custom server.
	ref := &PRRef{Owner: "owner", Repo: "repo", Number: 1}
	sha, err := client.GetHeadSHA(context.Background(), ref)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if sha != "abc123" {
		t.Errorf("got SHA %q, want %q", sha, "abc123")
	}
}
