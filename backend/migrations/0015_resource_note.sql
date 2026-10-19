-- Ressourcentyp NOTE: eine Datei aus Kairos Notizen (Notiz, PDF, Bild, Zeichnung), target ist ihr Pfad im Notizordner.
-- SQLite kann einen CHECK nicht ändern, deshalb wird die Tabelle neu angelegt (niemand verweist auf resources).
CREATE TABLE resources_new (
    id         TEXT PRIMARY KEY,
    task_id    TEXT REFERENCES tasks (id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects (id) ON DELETE CASCADE,
    type       TEXT NOT NULL CHECK (type IN ('FILE', 'FOLDER', 'URL', 'NOTE')),
    target     TEXT NOT NULL CHECK (length(trim(target)) > 0),
    label      TEXT NOT NULL DEFAULT '',
    created_at TEXT NOT NULL,
    CHECK ((task_id IS NULL) <> (project_id IS NULL))
);
INSERT INTO resources_new SELECT id, task_id, project_id, type, target, label, created_at FROM resources;
DROP TABLE resources;
ALTER TABLE resources_new RENAME TO resources;
CREATE INDEX resources_task_id    ON resources (task_id);
CREATE INDEX resources_project_id ON resources (project_id);
