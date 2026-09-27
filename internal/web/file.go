package web

import (
	"maps"
	"slices"

	"github.com/koh-sh/commd/internal/markdown"
)

// fileState is the file under review: its review state plus what the page
// shows of it, derived once since the source never changes.
type fileState struct {
	*markdown.ReviewState
	lines    []lineJSON    // the source (or diff) view
	rendered []sectionJSON // sections with HTML
}

// newFileState prepares f for review, rendering its sections once with image
// destinations passed through imageURL (see renderHTML).
func newFileState(f markdown.File, imageURL func(string) string) *fileState {
	fs := &fileState{
		ReviewState: markdown.NewReviewState(f),
		rendered:    renderSections(f.Doc, imageURL),
	}
	if f.Diff != nil {
		fs.lines = diffLines(f.Doc, f.Diff)
	} else {
		fs.lines = sourceLines(f.Doc)
	}
	return fs
}

// addComment stores a new comment. The page cannot choose its ID.
func (f *fileState) addComment(c markdown.ReviewComment) error {
	c.ID = ""
	return f.SaveComment(c)
}

// updateComment changes the comment with the given ID; an empty body deletes
// it (see markdown.ReviewState.SaveComment).
func (f *fileState) updateComment(id string, c markdown.ReviewComment) error {
	c.ID = id
	return f.SaveComment(c)
}

// search returns the IDs of the sections a search for query shows (see
// markdown.Document.SearchSections).
func (f *fileState) search(query string) []string {
	ids := slices.Sorted(maps.Keys(f.Doc.SearchSections(query)))
	if ids == nil {
		return []string{} // never null in JSON
	}
	return ids
}
