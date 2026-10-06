-- Zeiteinträge aus echten Zeitstempeln. Ein Eintrag ohne ended_at ist der
-- laufende Timer; davon gibt es höchstens einen.
--
-- Ein Eintrag gehört entweder zu einer Task (das Projekt ergibt sich aus der
-- Task) oder direkt zu einem Projekt. Er wird mit seinem Besitzer gelöscht.
--
-- Zeitpunkte werden hier mit fester Breite gespeichert
-- (YYYY-MM-DDTHH:MM:SS.mmmZ), damit der Textvergleich der Zeitreihenfolge
-- entspricht (CHECK, Sortierung, Bereichsfilter).
CREATE TABLE time_entries (
    id         TEXT PRIMARY KEY,
    task_id    TEXT REFERENCES tasks (id) ON DELETE CASCADE,
    project_id TEXT REFERENCES projects (id) ON DELETE CASCADE,
    started_at TEXT NOT NULL,
    ended_at   TEXT,
    source     TEXT NOT NULL CHECK (source IN ('MANUAL', 'VSCODIUM', 'AUTOMATIC')),
    CHECK ((task_id IS NULL) <> (project_id IS NULL)),
    CHECK (ended_at IS NULL OR ended_at >= started_at)
);

CREATE UNIQUE INDEX time_entries_one_running ON time_entries ((1)) WHERE ended_at IS NULL;
CREATE INDEX time_entries_task_id    ON time_entries (task_id);
CREATE INDEX time_entries_project_id ON time_entries (project_id);
CREATE INDEX time_entries_started_at ON time_entries (started_at);
