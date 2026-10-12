package github

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"kairo/internal/domain"
)

func TestInvolved(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("Authorization") != "Bearer tok" || r.URL.Query().Get("q") != "is:open is:issue involves:@me archived:false" {
			w.WriteHeader(http.StatusUnauthorized)
			_, _ = w.Write([]byte(`{"message":"Bad credentials"}`))
			return
		}
		_, _ = w.Write([]byte(`{"items":[{"title":"Bug","html_url":"https://github.com/me/kairo/issues/7","number":7,
			"repository_url":"https://api.github.com/repos/me/kairo","updated_at":"2026-10-10T08:00:00Z","labels":[{"name":"bug"}]}]}`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Token: func(context.Context) (string, error) { return "tok", nil }}

	is, err := c.Involved(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(is) != 1 || is[0].Repo != "me/kairo" || is[0].Number != 7 || is[0].Labels[0] != "bug" || is[0].URL != "https://github.com/me/kairo/issues/7" {
		t.Errorf("Issues = %+v", is)
	}

	c.Token = func(context.Context) (string, error) { return "falsch", nil }
	if _, err := c.Involved(context.Background()); !errors.Is(err, domain.ErrUnavailable) {
		t.Errorf("401: %v", err)
	}
}

func TestRepoIssuesSkipsPullRequests(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/me/kairo/issues" || r.URL.Query().Get("state") != "open" {
			w.WriteHeader(http.StatusNotFound)
			return
		}
		_, _ = w.Write([]byte(`[{"title":"Issue","html_url":"https://github.com/me/kairo/issues/1","number":1,"repository_url":"https://api.github.com/repos/me/kairo"},
			{"title":"PR","html_url":"https://github.com/me/kairo/pull/2","number":2,"repository_url":"https://api.github.com/repos/me/kairo","pull_request":{}}]`))
	}))
	defer srv.Close()
	c := &Client{BaseURL: srv.URL, HTTP: srv.Client(), Token: func(context.Context) (string, error) { return "tok", nil }}
	is, err := c.RepoIssues(context.Background(), "me/kairo")
	if err != nil || len(is) != 1 || is[0].Title != "Issue" {
		t.Errorf("RepoIssues = %+v, %v", is, err)
	}
}

func TestRepoFromRemote(t *testing.T) {
	for in, want := range map[string]string{
		"git@github.com:Mirac61/Kairo.git":           "Mirac61/Kairo",
		"ssh://git@github.com/bug-bazaar/bugless":    "bug-bazaar/bugless",
		"https://github.com/nestjs/starter.git":      "nestjs/starter",
		"https://github.com/o/r/":                    "o/r",
		"ssh://git@codeberg.org/user/privat-notizen": "",
		"https://github.com/nur-owner":               "",
	} {
		if got := RepoFromRemote(in); got != want {
			t.Errorf("RepoFromRemote(%q) = %q, want %q", in, got, want)
		}
	}
}
