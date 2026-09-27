package github

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	gh "github.com/google/go-github/v84/github"
	"github.com/koh-sh/commd/internal/diff"
	"github.com/koh-sh/commd/internal/markdown"
)

func TestMapComment(t *testing.T) {
	doc := &markdown.Document{
		Sections: []*markdown.Section{
			{ID: "S1", Title: "Introduction", StartLine: 3, EndLine: 10},
			{ID: "S2", Title: "Details", StartLine: 12, EndLine: 20},
		},
	}

	tests := []struct {
		name    string
		comment markdown.ReviewComment
		path    string
		want    *PRReviewComment
	}{
		{
			name: "line-level single line",
			comment: markdown.ReviewComment{
				SectionID: "S1",
				Action:    markdown.ActionSuggestion,
				Body:      "Fix typo",
				StartLine: 5,
			},
			path: "README.md",
			want: &PRReviewComment{
				Path: "README.md",
				Body: "**[suggestion]** Fix typo",
				Line: 5,
				Side: diff.SideRight,
			},
		},
		{
			name: "line-level range",
			comment: markdown.ReviewComment{
				SectionID:  "S1",
				Action:     markdown.ActionIssue,
				Decoration: markdown.DecorationBlocking,
				Body:       "Rewrite this section",
				StartLine:  5,
				EndLine:    8,
			},
			path: "README.md",
			want: &PRReviewComment{
				Path:      "README.md",
				Body:      "**[issue (blocking)]** Rewrite this section",
				Line:      8,
				StartLine: 5,
				Side:      diff.SideRight,
			},
		},
		{
			name: "custom side LEFT is preserved",
			comment: markdown.ReviewComment{
				SectionID: "S1",
				Action:    markdown.ActionNote,
				Body:      "removed line note",
				StartLine: 5,
				Side:      diff.SideLeft,
			},
			path: "README.md",
			want: &PRReviewComment{
				Path: "README.md",
				Body: "**[note]** removed line note",
				Line: 5,
				Side: diff.SideLeft,
			},
		},
		{
			name: "section-level maps to heading line",
			comment: markdown.ReviewComment{
				SectionID: "S2",
				Action:    markdown.ActionQuestion,
				Body:      "Is this section needed?",
			},
			path: "README.md",
			want: &PRReviewComment{
				Path: "README.md",
				Body: "**[question]** Is this section needed?",
				Line: 12,
				Side: diff.SideRight,
			},
		},
		{
			name: "overview comment returns nil",
			comment: markdown.ReviewComment{
				SectionID: markdown.OverviewSectionID,
				Action:    markdown.ActionNote,
				Body:      "General note",
			},
			path: "README.md",
			want: nil,
		},
		{
			name: "unknown section returns nil",
			comment: markdown.ReviewComment{
				SectionID: "S99",
				Action:    markdown.ActionNote,
				Body:      "Note",
			},
			path: "README.md",
			want: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := MapComment(tt.comment, tt.path, doc)
			if tt.want == nil {
				if got != nil {
					t.Errorf("MapComment() = %+v, want nil", got)
				}
				return
			}
			if got == nil {
				t.Fatal("MapComment() = nil, want non-nil")
			}
			if got.Path != tt.want.Path {
				t.Errorf("Path = %q, want %q", got.Path, tt.want.Path)
			}
			if got.Body != tt.want.Body {
				t.Errorf("Body = %q, want %q", got.Body, tt.want.Body)
			}
			if got.Line != tt.want.Line {
				t.Errorf("Line = %d, want %d", got.Line, tt.want.Line)
			}
			if got.StartLine != tt.want.StartLine {
				t.Errorf("StartLine = %d, want %d", got.StartLine, tt.want.StartLine)
			}
			if got.Side != tt.want.Side {
				t.Errorf("Side = %q, want %q", got.Side, tt.want.Side)
			}
		})
	}
}

func TestBuildPRReview(t *testing.T) {
	doc := &markdown.Document{
		Sections: []*markdown.Section{
			{ID: "S1", Title: "Intro", StartLine: 3, EndLine: 10},
		},
	}

	tests := []struct {
		name         string
		results      []markdown.FileResult
		event        string
		body         string
		wantComments int
		wantBody     string
	}{
		{
			name: "line and section comments become inline",
			results: []markdown.FileResult{{
				Path: "README.md", Doc: doc,
				Review: &markdown.ReviewResult{
					Comments: []markdown.ReviewComment{
						{SectionID: "S1", Action: markdown.ActionSuggestion, Body: "Fix", StartLine: 5},
						{SectionID: "S1", Action: markdown.ActionNote, Body: "Section note"},
					},
				},
			}},
			event:        "COMMENT",
			wantComments: 2,
		},
		{
			name: "overview comments are skipped",
			results: []markdown.FileResult{{
				Path: "README.md", Doc: doc,
				Review: &markdown.ReviewResult{
					Comments: []markdown.ReviewComment{
						{SectionID: markdown.OverviewSectionID, Action: markdown.ActionNote, Body: "General"},
					},
				},
			}},
			event:        "COMMENT",
			wantComments: 0,
		},
		{
			name: "multi-line comment sets StartLine and StartSide",
			results: []markdown.FileResult{{
				Path: "README.md", Doc: doc,
				Review: &markdown.ReviewResult{
					Comments: []markdown.ReviewComment{
						{SectionID: "S1", Action: markdown.ActionIssue, Body: "Fix range", StartLine: 5, EndLine: 8},
					},
				},
			}},
			event:        "COMMENT",
			wantComments: 1,
		},
		{
			name: "nil review in result is skipped",
			results: []markdown.FileResult{{
				Path:   "README.md",
				Doc:    doc,
				Review: nil,
			}},
			event:        "COMMENT",
			wantComments: 0,
		},
		{
			name:         "empty results",
			results:      nil,
			event:        "APPROVE",
			wantComments: 0,
		},
		{
			name:     "body is set when non-empty",
			results:  nil,
			event:    "APPROVE",
			body:     "Looks good!",
			wantBody: "Looks good!",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := BuildPRReview(tt.results, tt.event, tt.body)
			if got.GetEvent() != tt.event {
				t.Errorf("Event = %q, want %q", got.GetEvent(), tt.event)
			}
			if len(got.Comments) != tt.wantComments {
				t.Errorf("len(Comments) = %d, want %d", len(got.Comments), tt.wantComments)
			}
			if got.GetBody() != tt.wantBody {
				t.Errorf("Body = %q, want %q", got.GetBody(), tt.wantBody)
			}
			// Verify StartLine/StartSide are set for multi-line comments.
			if tt.name == "multi-line comment sets StartLine and StartSide" && len(got.Comments) == 1 {
				c := got.Comments[0]
				if c.GetStartLine() != 5 {
					t.Errorf("StartLine = %d, want 5", c.GetStartLine())
				}
				if c.GetStartSide() != diff.SideRight {
					t.Errorf("StartSide = %q, want %q", c.GetStartSide(), diff.SideRight)
				}
			}
		})
	}
}

func TestSubmitReview(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		wantErr    bool
	}{
		{
			name:       "successful submission",
			statusCode: 200,
		},
		{
			name:       "API error",
			statusCode: 422,
			wantErr:    true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mux := http.NewServeMux()
			mux.HandleFunc("POST /repos/owner/repo/pulls/1/reviews", func(w http.ResponseWriter, _ *http.Request) {
				if tt.statusCode != 200 {
					http.Error(w, `{"message":"error"}`, tt.statusCode)
					return
				}
				writeJSON(t, w, map[string]any{"id": 1})
			})
			srv := httptest.NewServer(mux)
			t.Cleanup(srv.Close)

			client := NewClientWithHTTP(srv.Client(), srv.URL+"/")
			ref := &PRRef{Owner: "owner", Repo: "repo", Number: 1}

			review := &gh.PullRequestReviewRequest{
				Event: new("COMMENT"),
				Body:  new("test"),
			}

			err := client.SubmitReview(context.Background(), ref, review)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
		})
	}
}
