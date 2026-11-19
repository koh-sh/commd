package markdown

import (
	"strconv"
	"strings"
	"unicode"
)

// EscapeControl returns s with every control character except tab replaced
// by its Go escape (`\n`, `\x1b`, `\u009b`, ...). Reviewed documents, file
// names and API data are written by someone other than the reviewer, so
// their text goes through this before it reaches a terminal or the
// line-oriented review output: an escape sequence cannot drive the terminal
// and a newline cannot start a line of its own.
func EscapeControl(s string) string {
	if !strings.ContainsFunc(s, isEscapedControl) {
		return s
	}
	var sb strings.Builder
	for _, r := range s {
		if !isEscapedControl(r) {
			sb.WriteRune(r)
			continue
		}
		q := strconv.QuoteRune(r)
		sb.WriteString(q[1 : len(q)-1]) // drop the surrounding quotes
	}
	return sb.String()
}

func isEscapedControl(r rune) bool {
	return r != '\t' && unicode.IsControl(r)
}
