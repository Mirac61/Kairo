-- Ressourcen sind Referenzen (Datei, Ordner, URL) auf Tasks oder Projekte.
-- Eine Ressource gehört genau einem Besitzer und wird mit ihm gelöscht.
CREATE TABLE resources (
    id         TEXT PRIMARY KEY,
    task_id    TEXT REFERENCES tasks (id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects (id) ON DELETE CASCADE,
    type       TEXT NOT NULL CHECK (type IN ('FILE', 'FOLDER', 'URL')),
    target     TEXT NOT NULL CHECK (length(trim(target)) > 0),
    label      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    CHECK ((task_id IS NULL) <> (project_id IS NULL))
);

CREATE INDEX resources_task_id    ON resources (task_id);
CREATE INDEX resources_project_id ON resources (project_id);
