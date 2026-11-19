package markdown

import "testing"

func TestEscapeControl(t *testing.T) {
	tests := []struct {
		name string
		in   string
		want string
	}{
		{name: "plain text unchanged", in: "docs/plan.md", want: "docs/plan.md"},
		{name: "multibyte unchanged", in: "計画 ✓", want: "計画 ✓"},
		{name: "tab kept", in: "a\tb", want: "a\tb"},
		{name: "newline escaped", in: "a\n[issue] forged", want: `a\n[issue] forged`},
		{name: "carriage return escaped", in: "a\rb", want: `a\rb`},
		{name: "OSC sequence neutralized", in: "a\x1b]0;T\x07.md", want: `a\x1b]0;T\a.md`},
		{name: "C1 CSI escaped", in: "\u009b2J", want: `\u009b2J`},
		{name: "DEL escaped", in: "a\x7f", want: `a\x7f`},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := EscapeControl(tt.in); got != tt.want {
				t.Errorf("EscapeControl(%q) = %q, want %q", tt.in, got, tt.want)
			}
		})
	}
}
