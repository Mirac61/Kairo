-- Ziele fürs Motivationsboard. Der Fortschritt wird aus Projekten, Zeiterfassung und Gewohnheiten berechnet.
CREATE TABLE goals (
    id         TEXT PRIMARY KEY,
    title      TEXT NOT NULL,
    why        TEXT NOT NULL DEFAULT '',
    image_path TEXT,
    kind       TEXT NOT NULL DEFAULT 'NONE',
    project_id TEXT REFERENCES projects (id) ON DELETE SET NULL,
    habit_id   TEXT REFERENCES habits (id) ON DELETE SET NULL,
    target     INTEGER NOT NULL DEFAULT 0,
    start_date TEXT NOT NULL,
    due_date   TEXT,
    archived   INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL
);
