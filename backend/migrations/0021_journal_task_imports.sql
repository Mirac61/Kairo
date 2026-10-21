ALTER TABLE tasks ADD COLUMN source_journal_date TEXT;
CREATE INDEX tasks_source_journal_date ON tasks (source_journal_date);

CREATE TABLE journal_task_imports (
    journal_date TEXT NOT NULL PRIMARY KEY,
    task_count   INTEGER NOT NULL CHECK (task_count > 0),
    imported_at  TEXT NOT NULL
);
