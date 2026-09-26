package web

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
)

// assetURL returns the imageURL function for a file under review: relative
// image paths are served from the file's directory by /assets/ (see
// serveAsset), authenticated by the token in the query since images cannot
// send headers. Other destinations (URLs, absolute paths) are kept.
func assetURL(seq int, token string) func(string) string {
	return func(dest string) string {
		u, err := url.Parse(dest)
		if err != nil || u.Scheme != "" || u.Host != "" || u.Path == "" || strings.HasPrefix(u.Path, "/") {
			return dest
		}
		return fmt.Sprintf("/assets/%d/%s?t=%s", seq, u.EscapedPath(), url.QueryEscape(token))
	}
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
	if !validToken(r.URL.Query().Get("t"), a.s.token) {
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

// openAsset opens a file relative to the directory of the file under review.
// os.Root keeps the lookup inside that directory, including through
// symlinks and "..".
func (s *session) openAsset(seq int, name string) (*os.File, error) {
	var dir string
	err := s.withFile(seq, func(f *fileState) error {
		dir = filepath.Dir(f.Path)
		return nil
	})
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, errNotFound)
	}
	defer root.Close()
	f, err := root.Open(filepath.FromSlash(name))
	if err != nil {
		return nil, fmt.Errorf("%s: %w", name, errNotFound)
	}
	return f, nil
}
