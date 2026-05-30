package github

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	gh "github.com/google/go-github/v84/github"
)

// Client wraps the go-github client for PR operations.
type Client struct {
	inner *gh.Client
}

// NewClient creates a GitHub client using available authentication.
// Priority: GITHUB_TOKEN env var > gh auth token command.
func NewClient() (*Client, error) {
	token, err := resolveToken()
	if err != nil {
		return nil, err
	}

	client := gh.NewClient(nil).WithAuthToken(token)
	if baseURL := os.Getenv("COMMD_GITHUB_API_URL"); baseURL != "" {
		parsed, err := client.BaseURL.Parse(baseURL)
		if err != nil {
			return nil, fmt.Errorf("invalid COMMD_GITHUB_API_URL %q: %w", baseURL, err)
		}
		client.BaseURL = parsed
	}
	return &Client{inner: client}, nil
}

// NewClientWithHTTP creates a Client with a custom HTTP client (for testing).
func NewClientWithHTTP(httpClient *http.Client, baseURL string) *Client {
	client := gh.NewClient(httpClient)
	if baseURL != "" {
		client.BaseURL, _ = client.BaseURL.Parse(baseURL)
	}
	return &Client{inner: client}
}

// resolveToken returns a GitHub token from environment or gh CLI.
func resolveToken() (string, error) {
	if token := strings.TrimSpace(os.Getenv("GITHUB_TOKEN")); token != "" {
		return token, nil
	}

	out, err := exec.Command("gh", "auth", "token").Output()
	if err == nil {
		token := strings.TrimSpace(string(out))
		if token != "" {
			return token, nil
		}
	}

	return "", fmt.Errorf("GitHub token not found. Set GITHUB_TOKEN or run 'gh auth login'")
}

// GetHeadSHA returns the head commit SHA for a pull request.
func (c *Client) GetHeadSHA(ctx context.Context, ref *PRRef) (string, error) {
	pr, _, err := c.inner.PullRequests.Get(ctx, ref.Owner, ref.Repo, ref.Number)
	if err != nil {
		return "", fmt.Errorf("getting PR: %w", err)
	}

	sha := pr.GetHead().GetSHA()
	// Defensive fallback: use ref name if SHA is empty.
	if sha == "" {
		sha = pr.GetHead().GetRef()
	}
	return sha, nil
}

// ListMDFiles returns all .md files changed/added in a PR with their patches.
// Deleted files are excluded.
func (c *Client) ListMDFiles(ctx context.Context, ref *PRRef) ([]PRFile, error) {
	var allFiles []*gh.CommitFile
	opts := &gh.ListOptions{PerPage: 100}

	for {
		files, resp, err := c.inner.PullRequests.ListFiles(ctx, ref.Owner, ref.Repo, ref.Number, opts)
		if err != nil {
			return nil, fmt.Errorf("listing PR files: %w", err)
		}
		allFiles = append(allFiles, files...)

		if resp.NextPage == 0 {
			break
		}
		opts.Page = resp.NextPage
	}

	var files []PRFile
	for _, f := range allFiles {
		name := f.GetFilename()
		if f.GetStatus() == "removed" {
			continue
		}
		if filepath.Ext(name) != ".md" {
			continue
		}
		files = append(files, PRFile{
			Path:  name,
			Patch: f.GetPatch(),
		})
	}

	return files, nil
}

// FetchFileContent fetches a file's content at the given ref (SHA or branch).
func (c *Client) FetchFileContent(ctx context.Context, ref *PRRef, path, headSHA string) ([]byte, error) {
	content, _, _, err := c.inner.Repositories.GetContents(
		ctx, ref.Owner, ref.Repo, path,
		&gh.RepositoryContentGetOptions{Ref: headSHA},
	)
	if err != nil {
		return nil, fmt.Errorf("fetching file %s: %w", path, err)
	}
	if content == nil {
		return nil, fmt.Errorf("fetching file %s: path is a directory, not a file", path)
	}

	// Files between 1 MB and 100 MB come back with an empty body and
	// encoding "none", which GetContent cannot decode. Fall back to the raw
	// download URL, which the API already resolves to the requested ref.
	if content.GetEncoding() == "none" {
		return c.downloadRawContent(ctx, content.GetDownloadURL(), path)
	}

	decoded, err := content.GetContent()
	if err != nil {
		return nil, fmt.Errorf("decoding file %s: %w", path, err)
	}

	return []byte(decoded), nil
}

// downloadRawContent fetches a file's bytes from its raw download URL,
// bypassing the 1 MB GetContents decode limit.
func (c *Client) downloadRawContent(ctx context.Context, url, path string) ([]byte, error) {
	if url == "" {
		return nil, fmt.Errorf("fetching file %s: no download URL for oversized file", path)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("downloading file %s: %w", path, err)
	}

	resp, err := c.inner.Client().Do(req)
	if err != nil {
		return nil, fmt.Errorf("downloading file %s: %w", path, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("downloading file %s: unexpected status %d", path, resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("reading file %s: %w", path, err)
	}

	return data, nil
}
