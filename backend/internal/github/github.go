// Package github liest offene Issues über die GitHub-REST-API.
// Das Token kommt aus GH_TOKEN/GITHUB_TOKEN oder von der gh-CLI; Kairo speichert keins.
package github

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"time"

	"kairo/internal/domain"
)

// Issue ist ein offenes Issue.
type Issue struct {
	Title     string
	URL       string // html_url, z. B. https://github.com/owner/repo/issues/12
	Repo      string // owner/repo
	Number    int
	Labels    []string
	UpdatedAt time.Time
}

// Client fragt die API ab. Token und BaseURL sind für Tests austauschbar.
type Client struct {
	BaseURL string
	Token   func(ctx context.Context) (string, error)
	HTTP    *http.Client
}

// New nutzt api.github.com und das Token von GhToken.
func New() *Client {
	return &Client{BaseURL: "https://api.github.com", Token: GhToken, HTTP: &http.Client{Timeout: 15 * time.Second}}
}

// ghPaths: Fallbacks, weil der Autostart (launchd/systemd) einen kurzen PATH hat.
var ghPaths = []string{"gh", "/opt/homebrew/bin/gh", "/usr/local/bin/gh", "/usr/bin/gh"}

// GhToken liefert GH_TOKEN bzw. GITHUB_TOKEN oder die Ausgabe von „gh auth token“.
func GhToken(ctx context.Context) (string, error) {
	for _, k := range []string{"GH_TOKEN", "GITHUB_TOKEN"} {
		if v := os.Getenv(k); v != "" {
			return v, nil
		}
	}
	for _, p := range ghPaths {
		bin, err := exec.LookPath(p)
		if err != nil {
			continue
		}
		out, err := exec.CommandContext(ctx, bin, "auth", "token").Output()
		if tok := strings.TrimSpace(string(out)); err == nil && tok != "" {
			return tok, nil
		}
		return "", fmt.Errorf("%w: gh ist nicht angemeldet, „gh auth login“ ausführen", domain.ErrUnavailable)
	}
	return "", fmt.Errorf("%w: gh-CLI nicht gefunden (https://cli.github.com), alternativ GH_TOKEN setzen", domain.ErrUnavailable)
}

// Involved liefert die offenen Issues aus allen Repos, an denen der Nutzer beteiligt ist
// (angelegt, zugewiesen, erwähnt oder kommentiert), höchstens 100, zuletzt aktualisierte zuerst.
func (c *Client) Involved(ctx context.Context) ([]Issue, error) {
	var body struct{ Items []item }
	if err := c.get(ctx, "/search/issues?q=is:open+is:issue+involves:@me+archived:false&sort=updated&per_page=100", &body); err != nil {
		return nil, err
	}
	return toIssues(body.Items), nil
}

// RepoIssues liefert die offenen Issues eines Repos (owner/repo), ohne Pull Requests, höchstens 100.
func (c *Client) RepoIssues(ctx context.Context, repo string) ([]Issue, error) {
	var items []item
	if err := c.get(ctx, "/repos/"+repo+"/issues?state=open&sort=updated&per_page=100", &items); err != nil {
		return nil, err
	}
	return toIssues(items), nil
}

// RepoOf liest das GitHub-Repo (owner/repo) aus dem Remote „origin“ des Ordners path;
// "" ohne Git, ohne origin oder bei einem anderen Anbieter.
func (c *Client) RepoOf(ctx context.Context, path string) string {
	if path == "~" || strings.HasPrefix(path, "~/") {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		path = filepath.Join(home, path[1:])
	}
	out, err := exec.CommandContext(ctx, "git", "-C", path, "remote", "get-url", "origin").Output()
	if err != nil {
		return ""
	}
	return RepoFromRemote(strings.TrimSpace(string(out)))
}

// RepoFromRemote macht aus git@github.com:o/r.git, ssh://git@github.com/o/r oder
// https://github.com/o/r.git „o/r“; andere Hosts ergeben "".
func RepoFromRemote(remote string) string {
	for _, prefix := range []string{"git@github.com:", "ssh://git@github.com/", "https://github.com/", "http://github.com/"} {
		if rest, ok := strings.CutPrefix(remote, prefix); ok {
			rest = strings.TrimSuffix(strings.TrimSuffix(rest, "/"), ".git")
			if parts := strings.Split(rest, "/"); len(parts) == 2 && parts[0] != "" && parts[1] != "" {
				return rest
			}
		}
	}
	return ""
}

type item struct {
	Title         string    `json:"title"`
	HTMLURL       string    `json:"html_url"`
	Number        int       `json:"number"`
	RepositoryURL string    `json:"repository_url"`
	UpdatedAt     time.Time `json:"updated_at"`
	PullRequest   *struct{} `json:"pull_request"`
	Labels        []struct {
		Name string `json:"name"`
	} `json:"labels"`
}

func toIssues(items []item) []Issue {
	out := make([]Issue, 0, len(items))
	for _, it := range items {
		if it.PullRequest != nil { // /repos/…/issues liefert auch Pull Requests
			continue
		}
		i := Issue{Title: it.Title, URL: it.HTMLURL, Number: it.Number, UpdatedAt: it.UpdatedAt,
			Repo: it.RepositoryURL[strings.Index(it.RepositoryURL, "/repos/")+len("/repos/"):]}
		for _, l := range it.Labels {
			i.Labels = append(i.Labels, l.Name)
		}
		out = append(out, i)
	}
	return out
}

// get ruft path mit Token auf und dekodiert die Antwort nach dst.
func (c *Client) get(ctx context.Context, path string, dst any) error {
	tok, err := c.Token(ctx)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+path, nil)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("Authorization", "Bearer "+tok)
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	res, err := c.HTTP.Do(req)
	if err != nil {
		return fmt.Errorf("%w: GitHub nicht erreichbar: %v", domain.ErrUnavailable, err)
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		var e struct{ Message string }
		_ = json.NewDecoder(res.Body).Decode(&e)
		return fmt.Errorf("%w: GitHub antwortet %d: %s", domain.ErrUnavailable, res.StatusCode, e.Message)
	}
	if err := json.NewDecoder(res.Body).Decode(dst); err != nil {
		return fmt.Errorf("github: Antwort lesen: %w", err)
	}
	return nil
}
