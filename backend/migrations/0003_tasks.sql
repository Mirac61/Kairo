CREATE TABLE tasks (
    id                TEXT PRIMARY KEY,
    project_id        TEXT REFERENCES projects (id) ON DELETE SET NULL,
    parent_task_id    TEXT REFERENCES tasks (id) ON DELETE CASCADE,
    title             TEXT NOT NULL CHECK (length(trim(title)) > 0),
    description       TEXT NOT NULL DEFAULT '',
    status            TEXT NOT NULL DEFAULT 'BACKLOG'
                      CHECK (status IN ('BACKLOG', 'PLANNED', 'IN_PROGRESS', 'PAUSED', 'COMPLETED', 'CANCELLED')),
    priority          TEXT NOT NULL DEFAULT 'MEDIUM'
                      CHECK (priority IN ('LOW', 'MEDIUM', 'HIGH', 'URGENT')),
    estimated_minutes INTEGER NOT NULL DEFAULT 0 CHECK (estimated_minutes >= 0),
    due_at            TEXT,
    planned_date      TEXT,
    planned_start_at  TEXT,
    created_at        TEXT NOT NULL,
    updated_at        TEXT NOT NULL,
    completed_at      TEXT,
    CHECK (planned_start_at IS NULL OR planned_date IS NOT NULL)
);

CREATE INDEX tasks_project_id     ON tasks (project_id);
CREATE INDEX tasks_parent_task_id ON tasks (parent_task_id);
CREATE INDEX tasks_planned_date   ON tasks (planned_date);
CREATE INDEX tasks_status         ON tasks (status);
