package githubsrc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"golang.org/x/mod/semver"
)

var (
	errNoAsset = errors.New("no windows release asset matched")
	errNoRel   = errors.New("github returned no release")
)

// Release is a GitHub tagged release plus the asset we would download.
type Release struct {
	TagName     string    `json:"tag_name"`
	Name        string    `json:"name"`
	HTMLURL     string    `json:"html_url"`
	PublishedAt time.Time `json:"published_at"`
	Assets      []Asset   `json:"assets"`
	Asset       Asset     `json:"asset"`
}

// Client talks to api.github.com.
type Client struct {
	HTTP  *http.Client
	Token string
}

func (c *Client) http() *http.Client {
	if c != nil && c.HTTP != nil {
		return c.HTTP
	}

	return &http.Client{Timeout: 20 * time.Second}
}

func (c *Client) token() string {
	if c == nil {
		return ""
	}

	return c.Token
}

// Latest fetches /repos/{ownerRepo}/releases/latest and selects an asset.
func (c *Client) Latest(ctx context.Context, ownerRepo, hostArch string) (*Release, error) {
	ownerRepo = NormalizeRepo(ownerRepo)
	if ownerRepo == "" || !strings.Contains(ownerRepo, "/") {
		return nil, errors.New("owner/repo required")
	}

	url := "https://api.github.com/repos/" + ownerRepo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "CaptainUpdater")
	if tok := c.token(); tok != "" {
		req.Header.Set("Authorization", "Bearer "+tok)
	}

	resp, err := c.http().Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return nil, err
	}

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("github %s: %s", resp.Status, truncate(string(body), 200))
	}

	var rel Release
	if err := json.Unmarshal(body, &rel); err != nil {
		return nil, err
	}

	if rel.TagName == "" {
		return nil, errNoRel
	}

	asset, err := Match(rel.Assets, hostArch)
	if err != nil {
		return nil, err
	}

	rel.Asset = asset

	return &rel, nil
}

// Newer reports whether remote is a greater semver than local.
func Newer(remote, local string) bool {
	r := canon(remote)
	l := canon(local)
	if !semver.IsValid(r) || !semver.IsValid(l) {
		rs := strings.TrimPrefix(r, "v")
		ls := strings.TrimPrefix(l, "v")

		return rs != ls && remote != "" && semver.Compare(r, l) > 0
	}

	return semver.Compare(r, l) > 0
}

// Display is the three-digit form used for last-applied / UI (strips a Windows
// PE fourth number and a leading v).
func Display(v string) string {
	v = strings.TrimSpace(v)
	if v == "" {
		return ""
	}

	return strings.TrimPrefix(canon(v), "v")
}

func canon(v string) string {
	v = strings.TrimSpace(v)
	v = strings.TrimPrefix(v, "v")
	v = strings.TrimPrefix(v, "V")
	if v == "" {
		return "v0.0.0"
	}

	main, meta := splitMeta(v)
	parts := strings.Split(main, ".")
	for len(parts) < 3 {
		parts = append(parts, "0")
	}

	if len(parts) > 3 {
		parts = parts[:3]
	}

	return "v" + strings.Join(parts, ".") + meta
}

func splitMeta(v string) (main, meta string) {
	build := ""
	if i := strings.Index(v, "+"); i >= 0 {
		build = v[i:]
		v = v[:i]
	}

	pre := ""
	if i := strings.Index(v, "-"); i >= 0 {
		pre = v[i:]
		v = v[:i]
	}

	return v, pre + build
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}

	return s[:n]
}
