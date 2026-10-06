CREATE TABLE calendar_events (
    id                 TEXT PRIMARY KEY,
    title              TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description        TEXT NOT NULL DEFAULT '',
    start_at           TEXT NOT NULL,
    end_at             TEXT NOT NULL,
    location           TEXT NOT NULL DEFAULT '',
    url                TEXT NOT NULL DEFAULT '',
    project_id         TEXT REFERENCES projects (id) ON DELETE SET NULL,
    task_id            TEXT REFERENCES tasks (id) ON DELETE SET NULL,
    recurrence_rule    TEXT,
    recurrence_exdates TEXT NOT NULL DEFAULT '[]' CHECK (json_valid(recurrence_exdates)),
    created_at         TEXT NOT NULL,
    updated_at         TEXT NOT NULL
);

CREATE INDEX calendar_events_start_at   ON calendar_events (start_at);
CREATE INDEX calendar_events_project_id ON calendar_events (project_id);
CREATE INDEX calendar_events_task_id    ON calendar_events (task_id);
