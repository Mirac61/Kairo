package api

import (
	"context"
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"testing"

	"kairo/internal/github"
	"kairo/internal/repository"
	"kairo/internal/service"
)

type fakeIssues []github.Issue

func (f fakeIssues) Involved(context.Context) ([]github.Issue, error) { return f, nil }
func (f fakeIssues) RepoIssues(_ context.Context, repo string) ([]github.Issue, error) {
	var out []github.Issue
	for _, i := range f {
		if i.Repo == repo {
			out = append(out, i)
		}
	}
	return out, nil
}

// RepoOf: der Ordnername steht für das Repo, „ohne-git“ hat keins.
func (f fakeIssues) RepoOf(_ context.Context, path string) string {
	if filepath.Base(path) == "ohne-git" {
		return ""
	}
	return "me/" + filepath.Base(path)
}

func TestGitHubImport(t *testing.T) {
	db, err := repository.Open(context.Background(), filepath.Join(t.TempDir(), "k.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { db.Close() })
	url := "https://github.com/me/kairo/issues/7"
	h := NewRouter(8742, testToken, "dev", http.NotFoundHandler(), Services{
		Projects:  service.NewProjectService(repository.NewProjectRepository(db), nil),
		Tasks:     service.NewTaskService(repository.NewTaskRepository(db), nil),
		Resources: service.NewResourceService(repository.NewResourceRepository(db), nil),
		GitHub:    fakeIssues{{Title: "Bug", URL: url, Repo: "me/kairo", Number: 7}},
	})
	list := func() (out []issueDTO) {
		rec := call(h, "GET", "/api/github/issues", "")
		if rec.Code != 200 {
			t.Fatalf("GET = %d %s", rec.Code, rec.Body)
		}
		_ = json.Unmarshal(rec.Body.Bytes(), &out)
		return out
	}
	if is := list(); len(is) != 1 || is[0].TaskID != nil || is[0].Labels == nil {
		t.Fatalf("vor Import = %+v", is)
	}

	body := `{"url":"` + url + `","title":"Bug","repo":"me/kairo","number":7}`
	rec := call(h, "POST", "/api/github/issues/import", body)
	if rec.Code != 201 {
		t.Fatalf("Import = %d %s", rec.Code, rec.Body)
	}
	var task taskDTO
	_ = json.Unmarshal(rec.Body.Bytes(), &task)
	if is := list(); is[0].TaskID == nil || *is[0].TaskID != task.ID || task.Title != "Bug" {
		t.Errorf("nach Import = %+v, Task %+v", is[0], task)
	}
	if rec := call(h, "POST", "/api/github/issues/import", body); rec.Code != 409 {
		t.Errorf("zweiter Import = %d", rec.Code)
	}
	if rec := call(h, "POST", "/api/github/issues/import", `{"url":"https://evil.example/x","title":"x"}`); rec.Code != 400 {
		t.Errorf("fremde URL = %d", rec.Code)
	}

	// Projekt, dessen Ordner zu me/kairo gehört, zeigt das importierte Issue samt Task.
	dir := filepath.Join(t.TempDir(), "kairo")
	noGit := filepath.Join(t.TempDir(), "ohne-git")
	for _, d := range []string{dir, noGit} {
		if err := os.Mkdir(d, 0o755); err != nil {
			t.Fatal(err)
		}
	}
	quote := func(s string) string { b, _ := json.Marshal(s); return string(b) } // Windows-Pfade escapen
	var resp struct {
		Repo   string     `json:"repo"`
		Issues []issueDTO `json:"issues"`
	}
	for path, want := range map[string]int{dir: 1, noGit: 0} {
		var p projectDTO
		_ = json.Unmarshal(call(h, "POST", "/api/projects", `{"name":"`+filepath.Base(path)+`","local_path":`+quote(path)+`}`).Body.Bytes(), &p)
		rec := call(h, "GET", "/api/projects/"+p.ID+"/github/issues", "")
		_ = json.Unmarshal(rec.Body.Bytes(), &resp)
		if rec.Code != 200 || len(resp.Issues) != want || (want == 1 && (resp.Repo != "me/kairo" || resp.Issues[0].TaskID == nil)) {
			t.Errorf("Projekt %s = %d %s", path, rec.Code, rec.Body)
		}
	}
}
