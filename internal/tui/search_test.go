package tui

import (
	"testing"
)

func TestSearchBar(t *testing.T) {
	tests := []struct {
		name      string
		setup     func(*SearchBar)
		wantQuery string
	}{
		{
			name:  "empty query after Open",
			setup: func(sb *SearchBar) { sb.Open() },
		},
		{
			name:  "empty query after Open then Close",
			setup: func(sb *SearchBar) { sb.Open(); sb.Close() },
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			sb := NewSearchBar()
			tt.setup(sb)

			if got := sb.Query(); got != tt.wantQuery {
				t.Errorf("Query() = %q, want %q", got, tt.wantQuery)
			}
		})
	}
}
