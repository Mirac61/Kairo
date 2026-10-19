package api

import (
	"context"
	"fmt"
	"net/http"
	"strings"
	"time"

	"kairo/internal/domain"
	"kairo/internal/github"
	"kairo/internal/service"
)

// IssueSource liefert GitHub-Issues und erkennt das Repo eines Projektordners.
type IssueSource interface {
	Involved(ctx context.Context) ([]github.Issue, error)
	RepoIssues(ctx context.Context, repo string) ([]github.Issue, error)
	RepoOf(ctx context.Context, path string) string
}

// githubHandlers legt Issues als Tasks an. Die Issue-URL hängt als URL-Ressource
// an der Task und dient zugleich als Merkmal „schon importiert“.
type githubHandlers struct {
	issues    IssueSource
	projects  ProjectService
	tasks     TaskService
	resources ResourceService
}

func (h githubHandlers) register(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/github/issues", h.list)
	mux.HandleFunc("GET /api/projects/{id}/github/issues", h.projectIssues)
	mux.HandleFunc("POST /api/github/issues/import", h.importIssue)
}

type issueDTO struct {
	Title     string   `json:"title"`
	URL       string   `json:"url"`
	Repo      string   `json:"repo"`
	Number    int      `json:"number"`
	Labels    []string `json:"labels"`
	UpdatedAt string   `json:"updated_at"`
	TaskID    *string  `json:"task_id"` // gesetzt, wenn schon als Task angelegt
}

// imported ordnet jeder Issue-URL, die als Ressource an einer Task hängt, die Task zu.
func (h githubHandlers) imported(ctx context.Context) (map[string]string, error) {
	rs, err := h.resources.List(ctx, domain.ResourceFilter{Type: domain.ResourceURL})
	if err != nil {
		return nil, err
	}
	m := map[string]string{}
	for _, r := range rs {
		if r.TaskID != nil {
			m[r.Target] = *r.TaskID
		}
	}
	return m, nil
}

// toDTOs ergänzt je Issue die Task, falls es schon importiert ist.
func (h githubHandlers) toDTOs(ctx context.Context, is []github.Issue) ([]issueDTO, error) {
	done, err := h.imported(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]issueDTO, 0, len(is))
	for _, i := range is {
		d := issueDTO{Title: i.Title, URL: i.URL, Repo: i.Repo, Number: i.Number, Labels: i.Labels,
			UpdatedAt: i.UpdatedAt.UTC().Format(time.RFC3339)}
		if id, ok := done[i.URL]; ok {
			d.TaskID = &id
		}
		if d.Labels == nil {
			d.Labels = []string{}
		}
		out = append(out, d)
	}
	return out, nil
}

// list: offene Issues aus allen Repos, an denen der Nutzer beteiligt ist.
func (h githubHandlers) list(w http.ResponseWriter, r *http.Request) {
	is, err := h.issues.Involved(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	out, err := h.toDTOs(r.Context(), is)
	if err != nil {
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, out)
}

// projectIssues: alle offenen Issues aus github_repo, sonst aus dem Repo hinter local_path
// (git remote origin, geht nur, wenn der Server den Ordner sieht). Ohne Repo ist die Liste leer.
func (h githubHandlers) projectIssues(w http.ResponseWriter, r *http.Request) {
	p, err := h.projects.Get(r.Context(), r.PathValue("id"))
	if err != nil {
		writeError(w, err)
		return
	}
	resp := struct {
		Repo   string     `json:"repo"`
		Issues []issueDTO `json:"issues"`
	}{Issues: []issueDTO{}}
	if p.GitHubRepo != nil {
		resp.Repo = *p.GitHubRepo
	} else if p.LocalPath != nil {
		resp.Repo = h.issues.RepoOf(r.Context(), *p.LocalPath)
	}
	if resp.Repo != "" {
		is, err := h.issues.RepoIssues(r.Context(), resp.Repo)
		if err == nil {
			resp.Issues, err = h.toDTOs(r.Context(), is)
		}
		if err != nil {
			writeError(w, err)
			return
		}
	}
	writeJSON(w, http.StatusOK, resp)
}

func (h githubHandlers) importIssue(w http.ResponseWriter, r *http.Request) {
	var in struct {
		URL       string  `json:"url"`
		Title     string  `json:"title"`
		Repo      string  `json:"repo"`
		Number    int     `json:"number"`
		ProjectID *string `json:"project_id"`
	}
	if !decodeJSON(w, r, &in) {
		return
	}
	if !strings.HasPrefix(in.URL, "https://github.com/") {
		writeError(w, fmt.Errorf("%w: url muss mit https://github.com/ beginnen", domain.ErrInvalid))
		return
	}
	done, err := h.imported(r.Context())
	if err != nil {
		writeError(w, err)
		return
	}
	if _, ok := done[in.URL]; ok {
		writeError(w, fmt.Errorf("%w: Issue ist schon als Task angelegt", domain.ErrConflict))
		return
	}
	t, err := h.tasks.Create(r.Context(), service.CreateTaskInput{Title: in.Title, ProjectID: in.ProjectID})
	if err != nil {
		writeError(w, err)
		return
	}
	taskID := t.ID
	if _, err := h.resources.Create(r.Context(), service.CreateResourceInput{
		TaskID: &taskID, Type: domain.ResourceURL, Target: in.URL, Label: fmt.Sprintf("%s#%d", in.Repo, in.Number),
	}); err != nil {
		// Ohne Link würde ein zweiter Import die Task doppeln.
		_ = h.tasks.Delete(r.Context(), taskID)
		writeError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, toTaskDTO(t))
}
