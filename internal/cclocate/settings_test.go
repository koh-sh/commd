package cclocate

import (
	"os"
	"path/filepath"
	"testing"
)

func TestResolvePlansDir(t *testing.T) {
	tests := []struct {
		name   string
		files  map[string]string // relative path from tmpDir → content
		wantFn func(tmpDir string) string
	}{
		{
			name:   "default when no settings exist",
			files:  nil,
			wantFn: func(string) string { return defaultPlansDir() },
		},
		{
			name: "from settings.local.json",
			files: map[string]string{
				".claude/settings.local.json": `{"plansDirectory": "/custom/plans"}`,
			},
			wantFn: func(string) string { return "/custom/plans" },
		},
		{
			name: "from settings.json",
			files: map[string]string{
				".claude/settings.json": `{"plansDirectory": "/project/plans"}`,
			},
			wantFn: func(string) string { return "/project/plans" },
		},
		{
			name: "relative path resolved from cwd",
			files: map[string]string{
				".claude/settings.json": `{"plansDirectory": "plans"}`,
			},
			wantFn: func(tmpDir string) string { return filepath.Join(tmpDir, "plans") },
		},
		{
			name: "local settings take priority over regular settings",
			files: map[string]string{
				".claude/settings.local.json": `{"plansDirectory": "/local/plans"}`,
				".claude/settings.json":       `{"plansDirectory": "/regular/plans"}`,
			},
			wantFn: func(string) string { return "/local/plans" },
		},
		{
			name: "broken local JSON falls back to valid settings",
			files: map[string]string{
				".claude/settings.local.json": `{broken`,
				".claude/settings.json":       `{"plansDirectory": "/fallback/plans"}`,
			},
			wantFn: func(string) string { return "/fallback/plans" },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tmpDir := t.TempDir()

			for relPath, content := range tt.files {
				absPath := filepath.Join(tmpDir, relPath)
				if err := os.MkdirAll(filepath.Dir(absPath), 0o755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(absPath, []byte(content), 0o644); err != nil {
					t.Fatal(err)
				}
			}

			got := ResolvePlansDir(tmpDir)
			want := tt.wantFn(tmpDir)
			if got != want {
				t.Errorf("ResolvePlansDir() = %q, want %q", got, want)
			}
		})
	}
}
