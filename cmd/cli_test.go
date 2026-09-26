package cmd

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/alecthomas/kong"
	ghclient "github.com/koh-sh/commd/internal/github"
	"github.com/koh-sh/commd/internal/markdown"
)

func TestVersionCmdRun(t *testing.T) {
	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	v := &VersionCmd{}
	err := v.Run(kong.Vars{"version": "1.2.3"})

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatal(err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if output != "commd version 1.2.3\n" {
		t.Errorf("output = %q, want %q", output, "commd version 1.2.3\n")
	}
}

func TestCommandValidate(t *testing.T) {
	type validator interface{ Validate() error }
	tests := []struct {
		name    string
		cmd     validator
		wantErr string
	}{
		// LocateCmd: requires --transcript or --stdin (paths are not stat'd here).
		{name: "locate no args", cmd: &LocateCmd{}, wantErr: "--transcript or --stdin is required"},
		{name: "locate transcript", cmd: &LocateCmd{Transcript: "any/path.jsonl"}},
		{name: "locate stdin", cmd: &LocateCmd{Stdin: true}},
		{name: "locate transcript and stdin", cmd: &LocateCmd{Transcript: "any/path.jsonl", Stdin: true}, wantErr: "--transcript and --stdin are mutually exclusive"},

		// ReviewCmd: requires --output-path when --output=file.
		{name: "review file with path", cmd: &ReviewCmd{Files: []string{"any.md"}, Output: "file", OutputPath: "any/path"}},
		{name: "review file without path", cmd: &ReviewCmd{Files: []string{"any.md"}, Output: "file"}, wantErr: "--output-path is required"},
		{name: "review stdout", cmd: &ReviewCmd{Files: []string{"any.md"}, Output: "stdout"}},
		{name: "review clipboard", cmd: &ReviewCmd{Files: []string{"any.md"}, Output: "clipboard"}},
		// ReviewCmd: exactly one <file> unless --diff; --base and --track-viewed are mode-specific.
		{name: "review no file", cmd: &ReviewCmd{Output: "stdout"}, wantErr: "expected exactly one <file>"},
		{name: "review two files", cmd: &ReviewCmd{Files: []string{"a.md", "b.md"}, Output: "stdout"}, wantErr: "expected exactly one <file>"},
		{name: "review base without diff", cmd: &ReviewCmd{Files: []string{"a.md"}, Output: "stdout", Base: "main"}, wantErr: "--base requires --diff"},
		{name: "review diff no file", cmd: &ReviewCmd{Diff: true, Output: "stdout"}},
		{name: "review diff many files", cmd: &ReviewCmd{Diff: true, Files: []string{"a.md", "b.md"}, Output: "stdout", Base: "main"}},
		{name: "review diff with track-viewed", cmd: &ReviewCmd{Diff: true, TrackViewed: true, Output: "stdout"}, wantErr: "--track-viewed cannot be combined with --diff"},
		// ReviewCmd: --port and --no-open only apply to --web.
		{name: "review web with port and no-open", cmd: &ReviewCmd{Files: []string{"a.md"}, Output: "stdout", Web: true, Port: 8080, NoOpen: true}},
		{name: "review web diff", cmd: &ReviewCmd{Diff: true, Output: "stdout", Web: true}},
		{name: "review port without web", cmd: &ReviewCmd{Files: []string{"a.md"}, Output: "stdout", Port: 8080}, wantErr: "--port and --no-open require --web"},
		{name: "review no-open without web", cmd: &ReviewCmd{Files: []string{"a.md"}, Output: "stdout", NoOpen: true}, wantErr: "--port and --no-open require --web"},
		{name: "review port out of range", cmd: &ReviewCmd{Files: []string{"a.md"}, Output: "stdout", Web: true, Port: 70000}, wantErr: "--port must be between 0 and 65535"},

		// PRCmd: requires a parseable GitHub PR URL.
		{name: "pr valid url", cmd: &PRCmd{URL: "https://github.com/owner/repo/pull/1"}},
		{name: "pr invalid url", cmd: &PRCmd{URL: "not-a-url"}, wantErr: "invalid PR URL"},
		{name: "pr empty url", cmd: &PRCmd{URL: ""}, wantErr: "invalid PR URL"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := tt.cmd.Validate()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want to contain %q", err, tt.wantErr)
			}
		})
	}
}

func TestLocateCmdRunWithTranscript(t *testing.T) {
	// Create a valid transcript JSONL with a plan file reference
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	// Create plan file
	planFile := filepath.Join(plansDir, "test-plan.md")
	if err := os.WriteFile(planFile, []byte("# Plan"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create settings
	settingsDir := filepath.Join(tmpDir, ".claude")
	settingsJSON := `{"plansDirectory":"` + plansDir + `"}`
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.local.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create transcript in the correct format:
	// {"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"..."}}]}}
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	transcriptLine := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"` + planFile + `"}}]}}`
	if err := os.WriteFile(transcriptFile, []byte(transcriptLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Capture stdout
	old := os.Stdout
	r, w, _ := os.Pipe()
	os.Stdout = w

	l := &LocateCmd{
		Transcript: transcriptFile,
		CWD:        tmpDir,
	}
	err := l.Run()

	w.Close()
	os.Stdout = old

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(r); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if output == "" {
		t.Error("expected plan file path in output")
	}
}

func TestLocateCmdRunNoPlanFound(t *testing.T) {
	tmpDir := t.TempDir()

	// Create empty transcript
	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	if err := os.WriteFile(transcriptFile, []byte(`{"type":"other"}`+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	l := &LocateCmd{
		Transcript: transcriptFile,
		CWD:        tmpDir,
	}
	err := l.Run()
	if err == nil {
		t.Fatal("expected error when no plan found")
	}
	if !strings.Contains(err.Error(), "no plan") {
		t.Errorf("error = %q, want to contain 'no plan'", err.Error())
	}
}

func TestWriteReviewOutput(t *testing.T) {
	t.Run("stdout", func(t *testing.T) {
		old := os.Stdout
		r, w, _ := os.Pipe()
		os.Stdout = w

		err := writeReviewOutput("hello", "stdout", "")

		w.Close()
		os.Stdout = old

		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(r); err != nil {
			t.Fatal(err)
		}
		if buf.String() != "hello" {
			t.Errorf("output = %q, want %q", buf.String(), "hello")
		}
	})

	t.Run("file write success", func(t *testing.T) {
		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "output.txt")
		// Create the file first so WriteFile can succeed
		if err := os.WriteFile(outFile, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}

		err := writeReviewOutput("review content", "file", outFile)
		if err != nil {
			t.Fatal(err)
		}

		got, err := os.ReadFile(outFile)
		if err != nil {
			t.Fatal(err)
		}
		if string(got) != "review content" {
			t.Errorf("file content = %q, want %q", string(got), "review content")
		}
	})

	t.Run("file deleted falls back", func(t *testing.T) {
		// Simulate hook timeout scenario: file was created then deleted.
		tmpDir := t.TempDir()
		outFile := filepath.Join(tmpDir, "output.txt")
		// File does not exist at this path (never created), simulating deletion.

		oldErr := os.Stderr
		r, w, _ := os.Pipe()
		os.Stderr = w

		err := writeReviewOutput("fallback content", "file", outFile)

		w.Close()
		os.Stderr = oldErr

		if err != nil {
			t.Fatal(err)
		}

		var buf bytes.Buffer
		if _, err := buf.ReadFrom(r); err != nil {
			t.Fatal(err)
		}
		stderr := buf.String()
		if !strings.Contains(stderr, "was deleted") {
			t.Errorf("stderr should mention file was deleted, got %q", stderr)
		}
		// File should NOT be created by the fallback path.
		if _, err := os.Stat(outFile); err == nil {
			t.Error("output file should not exist after fallback")
		}
	})

	t.Run("file write error returns error", func(t *testing.T) {
		// Create a read-only directory to cause a permission error
		tmpDir := t.TempDir()
		readOnlyDir := filepath.Join(tmpDir, "readonly")
		if err := os.MkdirAll(readOnlyDir, 0o755); err != nil {
			t.Fatal(err)
		}
		outFile := filepath.Join(readOnlyDir, "output.txt")
		// Create the file, then make directory read-only
		if err := os.WriteFile(outFile, []byte(""), 0o644); err != nil {
			t.Fatal(err)
		}
		if err := os.Chmod(outFile, 0o000); err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = os.Chmod(outFile, 0o644) })

		err := writeReviewOutput("review", "file", outFile)
		if err == nil {
			t.Fatal("expected error for permission denied")
		}
		if !strings.Contains(err.Error(), "writing output file") {
			t.Errorf("error = %q, want to contain 'writing output file'", err.Error())
		}
	})
}

func TestReviewCmdRunFileNotFound(t *testing.T) {
	r := &ReviewCmd{
		Files: []string{"/nonexistent/path/plan.md"},
	}
	err := r.Run()
	if err == nil {
		t.Fatal("expected error for nonexistent plan file")
	}
	if !strings.Contains(err.Error(), "reading file") {
		t.Errorf("error = %q, want to contain 'reading file'", err.Error())
	}
}

func TestLocateCmdRunStdinParseError(t *testing.T) {
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = w.WriteString("not valid json")
	w.Close()
	os.Stdin = r

	l := &LocateCmd{Stdin: true}
	err := l.Run()
	os.Stdin = oldStdin

	if err == nil {
		t.Fatal("expected error for invalid stdin JSON")
	}
	if !strings.Contains(err.Error(), "parsing") {
		t.Errorf("error = %q, want to contain 'parsing'", err.Error())
	}
}

func TestLocateCmdRunLocateError(t *testing.T) {
	tmpDir := t.TempDir()

	l := &LocateCmd{
		Transcript: "/nonexistent/transcript.jsonl",
		CWD:        tmpDir,
	}
	err := l.Run()
	if err == nil {
		t.Fatal("expected error for nonexistent transcript")
	}
	if !strings.Contains(err.Error(), "locating plan file") {
		t.Errorf("error = %q, want to contain 'locating plan file'", err.Error())
	}
}

func TestReviewCmdRunNoTerminal(t *testing.T) {
	tmpDir := t.TempDir()
	planFile := filepath.Join(tmpDir, "plan.md")
	if err := os.WriteFile(planFile, []byte("# Plan\n\n## Step 1\n\nContent.\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Use a pipe with Ctrl+C as input to prevent bubbletea from falling back
	// to /dev/tty, which would start a real TUI and hang the test.
	pr, pw, _ := os.Pipe()
	_, _ = pw.Write([]byte{3}) // Ctrl+C to quit immediately
	pw.Close()

	r := &ReviewCmd{
		Files:   []string{planFile},
		Output:  "stdout",
		teaOpts: []tea.ProgramOption{tea.WithInput(pr)},
	}
	err := r.Run()
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
}

func TestHookCmdRunExit(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		envSkip  bool
		wantCode int
	}{
		{
			name:     "parse error",
			input:    "not valid json",
			wantCode: 0,
		},
		{
			name:     "non-plan mode",
			input:    `{"session_id":"test","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","hook_event_name":"PostToolUse","permission_mode":"default","tool_name":"Write","tool_input":{"file_path":"/tmp/file.go"}}`,
			wantCode: 0,
		},
		{
			name:     "skip env",
			input:    `{"session_id":"test","transcript_path":"/tmp/t.jsonl","cwd":"/tmp","hook_event_name":"PostToolUse","permission_mode":"plan","tool_name":"Write","tool_input":{"file_path":"/tmp/file.go"}}`,
			envSkip:  true,
			wantCode: 0,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.envSkip {
				t.Setenv("CC_PLAN_REVIEW_SKIP", "1")
			}
			h := &HookCmd{Spawner: "auto", Theme: "dark"}
			code := h.runExit(strings.NewReader(tt.input))
			if code != tt.wantCode {
				t.Errorf("exit code = %d, want %d", code, tt.wantCode)
			}
		})
	}
}

func TestLocateCmdRunStdinMode(t *testing.T) {
	tmpDir := t.TempDir()
	plansDir := filepath.Join(tmpDir, ".claude", "plans")
	if err := os.MkdirAll(plansDir, 0o755); err != nil {
		t.Fatal(err)
	}

	planFile := filepath.Join(plansDir, "plan.md")
	if err := os.WriteFile(planFile, []byte("# Plan"), 0o644); err != nil {
		t.Fatal(err)
	}

	settingsDir := filepath.Join(tmpDir, ".claude")
	settingsJSON := `{"plansDirectory":"` + plansDir + `"}`
	if err := os.WriteFile(filepath.Join(settingsDir, "settings.local.json"), []byte(settingsJSON), 0o644); err != nil {
		t.Fatal(err)
	}

	transcriptFile := filepath.Join(tmpDir, "transcript.jsonl")
	transcriptLine := `{"type":"assistant","message":{"role":"assistant","content":[{"type":"tool_use","name":"Write","input":{"file_path":"` + planFile + `"}}]}}`
	if err := os.WriteFile(transcriptFile, []byte(transcriptLine+"\n"), 0o644); err != nil {
		t.Fatal(err)
	}

	// Create stdin with hook input JSON
	hookInput := `{"session_id":"test","transcript_path":"` + transcriptFile + `","cwd":"` + tmpDir + `"}`
	oldStdin := os.Stdin
	r, w, _ := os.Pipe()
	_, _ = w.WriteString(hookInput)
	w.Close()
	os.Stdin = r

	// Capture stdout
	oldStdout := os.Stdout
	rOut, wOut, _ := os.Pipe()
	os.Stdout = wOut

	l := &LocateCmd{
		Stdin: true,
	}
	err := l.Run()

	wOut.Close()
	os.Stdout = oldStdout
	os.Stdin = oldStdin

	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	var buf bytes.Buffer
	if _, err := buf.ReadFrom(rOut); err != nil {
		t.Fatal(err)
	}
	output := buf.String()

	if output == "" {
		t.Error("expected plan file path in output")
	}
}

// Validate normally guards against invalid URLs, but Run is also defensive in
// case it is called directly (e.g. from tests). nil client is safe here because
// ParsePRURL returns an error before client is touched.
func TestPRCmdRunInvalidURL(t *testing.T) {
	p := &PRCmd{URL: "not-a-url"}
	if err := p.Run(nil); err == nil {
		t.Fatal("expected error for invalid URL")
	}
}

func TestPRCmdRunNoMDFiles(t *testing.T) {
	srv := prTestServer(t, []map[string]string{
		{"filename": "main.go", "status": "modified"},
	}, "")

	client := ghclient.NewClientWithHTTP(srv.Client(), srv.URL+"/")
	p := &PRCmd{URL: "https://github.com/owner/repo/pull/1"}
	err := p.Run(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPRCmdRunFileNotInPR(t *testing.T) {
	srv := prTestServer(t, []map[string]string{
		{"filename": "README.md", "status": "modified"},
	}, "")

	client := ghclient.NewClientWithHTTP(srv.Client(), srv.URL+"/")
	p := &PRCmd{
		URL:  "https://github.com/owner/repo/pull/1",
		File: "missing.md",
	}
	err := p.Run(client)
	if err == nil {
		t.Fatal("expected error for missing file")
	}
	if !strings.Contains(err.Error(), "not found in PR") {
		t.Errorf("error = %q, want to contain 'not found in PR'", err.Error())
	}
}

func TestPRCmdRunWithFileFlag(t *testing.T) {
	srv := prTestServer(t, []map[string]string{
		{"filename": "README.md", "status": "modified"},
	}, "# Test\n\n## Section\n\nContent.\n")

	client := ghclient.NewClientWithHTTP(srv.Client(), srv.URL+"/")

	// Use Ctrl+C to quit the TUI immediately
	pr, pw, _ := os.Pipe()
	_, _ = pw.Write([]byte{3}) // Ctrl+C
	pw.Close()

	p := &PRCmd{
		URL:     "https://github.com/owner/repo/pull/1",
		File:    "README.md",
		Theme:   "dark",
		teaOpts: []tea.ProgramOption{tea.WithInput(pr)},
	}
	err := p.Run(client)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPRCmdSubmitReviewComment(t *testing.T) {
	srv := prTestServer(t, nil, "")

	client := ghclient.NewClientWithHTTP(srv.Client(), srv.URL+"/")
	ref := &ghclient.PRRef{Owner: "owner", Repo: "repo", Number: 1}

	doc := &markdown.Document{
		Sections: []*markdown.Section{
			{ID: "S1", Title: "Intro", StartLine: 3, EndLine: 10},
		},
	}
	results := []ghclient.FileReviewResult{{
		Path: "README.md",
		Doc:  doc,
		Review: &markdown.ReviewResult{
			Comments: []markdown.ReviewComment{
				{SectionID: "S1", Action: markdown.ActionSuggestion, Body: "Fix typo", StartLine: 5},
			},
		},
	}}

	p := &PRCmd{Theme: "dark"}
	err := p.submitReview(context.Background(), client, ref, results, "COMMENT", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPRCmdSubmitReviewApprove(t *testing.T) {
	srv := prTestServer(t, nil, "")

	client := ghclient.NewClientWithHTTP(srv.Client(), srv.URL+"/")
	ref := &ghclient.PRRef{Owner: "owner", Repo: "repo", Number: 1}

	p := &PRCmd{Theme: "dark"}
	err := p.submitReview(context.Background(), client, ref, nil, "APPROVE", "")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestPRCmdSubmitReviewError(t *testing.T) {
	mux := http.NewServeMux()
	mux.HandleFunc("POST /repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"message":"error"}`, http.StatusUnprocessableEntity)
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)

	client := ghclient.NewClientWithHTTP(srv.Client(), srv.URL+"/")
	ref := &ghclient.PRRef{Owner: "owner", Repo: "repo", Number: 1}

	doc := &markdown.Document{
		Sections: []*markdown.Section{
			{ID: "S1", Title: "Intro", StartLine: 3},
		},
	}
	results := []ghclient.FileReviewResult{{
		Path: "README.md",
		Doc:  doc,
		Review: &markdown.ReviewResult{
			Comments: []markdown.ReviewComment{
				{SectionID: "S1", Action: markdown.ActionNote, Body: "note"},
			},
		},
	}}

	p := &PRCmd{Theme: "dark"}
	err := p.submitReview(context.Background(), client, ref, results, "COMMENT", "")
	if err == nil {
		t.Fatal("expected error")
	}
}

// prTestServer creates a mock GitHub API server for PR tests.
func prTestServer(t *testing.T, files []map[string]string, fileContent string) *httptest.Server {
	t.Helper()
	mux := http.NewServeMux()

	// List PR files
	mux.HandleFunc("GET /repos/owner/repo/pulls/1/files", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(files); err != nil {
			t.Fatalf("encoding files: %v", err)
		}
	})

	// Get PR (for head SHA)
	mux.HandleFunc("GET /repos/owner/repo/pulls/1", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"head": map[string]any{"sha": "abc123", "ref": "feature"},
		}); err != nil {
			t.Fatalf("encoding PR: %v", err)
		}
	})

	// Get file contents
	if fileContent != "" {
		mux.HandleFunc("GET /repos/owner/repo/contents/", func(w http.ResponseWriter, _ *http.Request) {
			encoded := base64.StdEncoding.EncodeToString([]byte(fileContent))
			w.Header().Set("Content-Type", "application/json")
			if err := json.NewEncoder(w).Encode(map[string]any{
				"type": "file", "encoding": "base64", "content": encoded,
			}); err != nil {
				t.Fatalf("encoding content: %v", err)
			}
		})
	}

	// Create review
	mux.HandleFunc("POST /repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{"id": 1}); err != nil {
			t.Fatalf("encoding review: %v", err)
		}
	})

	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// TestCLIDefaultCommand verifies that `commd <file>` resolves to the review
// subcommand (default:"withargs") while named subcommands keep precedence.
func TestCLIDefaultCommand(t *testing.T) {
	tests := []struct {
		name       string
		args       []string
		wantCmd    string
		wantFile   string
		wantOutput string
		wantErr    string
	}{
		{name: "explicit review", args: []string{"review", "doc.md"}, wantCmd: "review <file>", wantFile: "doc.md"},
		{name: "implicit review", args: []string{"doc.md"}, wantCmd: "review <file>", wantFile: "doc.md"},
		{name: "implicit review with leading flag", args: []string{"--output", "stdout", "doc.md"}, wantCmd: "review <file>", wantFile: "doc.md", wantOutput: "stdout"},
		{name: "implicit review with trailing flag", args: []string{"doc.md", "--output", "stdout"}, wantCmd: "review <file>", wantFile: "doc.md", wantOutput: "stdout"},
		{name: "implicit diff review without file", args: []string{"--diff"}, wantCmd: "review"},
		{name: "diff review with files", args: []string{"review", "--diff", "a.md", "b.md"}, wantCmd: "review <file>", wantFile: "a.md,b.md"},
		{name: "named command takes precedence", args: []string{"version"}, wantCmd: "version"},
		{name: "pr command", args: []string{"pr", "https://github.com/owner/repo/pull/1"}, wantCmd: "pr <url>"},
		{name: "no args", args: nil, wantErr: "expected exactly one <file>"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			var cli CLI
			parser, err := kong.New(&cli, kong.Name("commd"))
			if err != nil {
				t.Fatal(err)
			}
			ctx, err := parser.Parse(tt.args)
			if tt.wantErr != "" {
				if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
					t.Errorf("err = %v, want to contain %q", err, tt.wantErr)
				}
				return
			}
			if err != nil {
				t.Fatalf("Parse(%v): %v", tt.args, err)
			}
			if got := ctx.Command(); got != tt.wantCmd {
				t.Errorf("Command() = %q, want %q", got, tt.wantCmd)
			}
			if got := strings.Join(cli.Review.Files, ","); got != tt.wantFile {
				t.Errorf("Review.Files = %q, want %q", got, tt.wantFile)
			}
			if tt.wantOutput != "" && cli.Review.Output != tt.wantOutput {
				t.Errorf("Review.Output = %q, want %q", cli.Review.Output, tt.wantOutput)
			}
		})
	}
}

// initGitRepo creates a git repository with doc.md committed and returns its path.
func initGitRepo(t *testing.T) string {
	t.Helper()
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		c := exec.Command("git", args...)
		c.Dir = dir
		c.Env = append(os.Environ(),
			"GIT_AUTHOR_NAME=test", "GIT_AUTHOR_EMAIL=test@example.com",
			"GIT_COMMITTER_NAME=test", "GIT_COMMITTER_EMAIL=test@example.com",
		)
		if out, err := c.CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q")
	if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte("# Doc\n\n## S\n\nold\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "-A")
	run("commit", "-q", "--no-verify", "-m", "chore: init")
	return dir
}

func TestReviewCmdRunDiff(t *testing.T) {
	// Ctrl+C quits the TUI immediately when it does open.
	ctrlC := func() []tea.ProgramOption {
		pr, pw, _ := os.Pipe()
		_, _ = pw.Write([]byte{3})
		pw.Close()
		return []tea.ProgramOption{tea.WithInput(pr)}
	}

	tests := []struct {
		name    string
		setup   func(t *testing.T) string // returns working directory
		cmd     ReviewCmd
		wantErr string
	}{
		{
			name:    "not a git repository",
			setup:   func(t *testing.T) string { return t.TempDir() },
			cmd:     ReviewCmd{Diff: true, Files: []string{"doc.md"}, Output: "stdout"},
			wantErr: "not a git repository",
		},
		{
			name:    "unknown base ref",
			setup:   initGitRepo,
			cmd:     ReviewCmd{Diff: true, Base: "no-such-ref", Files: []string{"doc.md"}, Output: "stdout"},
			wantErr: "unknown git ref",
		},
		{
			name:  "unchanged file is skipped without error",
			setup: initGitRepo,
			cmd:   ReviewCmd{Diff: true, Files: []string{"doc.md"}, Output: "stdout"},
		},
		{
			name: "changed file opens the TUI",
			setup: func(t *testing.T) string {
				dir := initGitRepo(t)
				if err := os.WriteFile(filepath.Join(dir, "doc.md"), []byte("# Doc\n\n## S\n\nnew\n"), 0o644); err != nil {
					t.Fatal(err)
				}
				return dir
			},
			cmd: ReviewCmd{Diff: true, Files: []string{"doc.md"}, Output: "stdout"},
		},
		{
			name:  "no changed files without explicit file",
			setup: initGitRepo,
			cmd:   ReviewCmd{Diff: true, Output: "stdout"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Chdir(tt.setup(t))
			r := tt.cmd
			r.teaOpts = ctrlC()
			err := r.Run()
			if tt.wantErr == "" {
				if err != nil {
					t.Fatalf("unexpected error: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tt.wantErr) {
				t.Errorf("err = %v, want to contain %q", err, tt.wantErr)
			}
		})
	}
}
