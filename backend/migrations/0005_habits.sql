CREATE TABLE habits (
    id               TEXT PRIMARY KEY,
    name             TEXT NOT NULL CHECK (length(trim(name)) > 0),
    description      TEXT NOT NULL DEFAULT '',
    frequency_type   TEXT NOT NULL
                     CHECK (frequency_type IN ('DAILY', 'WEEKLY', 'SPECIFIC_WEEKDAYS', 'TIMES_PER_WEEK')),
    frequency_config TEXT NOT NULL DEFAULT '{}' CHECK (json_valid(frequency_config)),
    target_value     INTEGER CHECK (target_value IS NULL OR target_value > 0),
    unit             TEXT NOT NULL DEFAULT '',
    preferred_time   TEXT CHECK (preferred_time IS NULL OR preferred_time GLOB '[0-2][0-9]:[0-5][0-9]'),
    start_date       TEXT NOT NULL,
    end_date         TEXT,
    active           INTEGER NOT NULL DEFAULT 1 CHECK (active IN (0, 1)),
    created_at       TEXT NOT NULL,
    CHECK (end_date IS NULL OR end_date >= start_date)
);

CREATE TABLE habit_completions (
    id           TEXT PRIMARY KEY,
    habit_id     TEXT NOT NULL REFERENCES habits (id) ON DELETE CASCADE,
    date         TEXT NOT NULL,
    value        INTEGER CHECK (value IS NULL OR value >= 0),
    completed_at TEXT NOT NULL,
    note         TEXT NOT NULL DEFAULT ''
);

CREATE UNIQUE INDEX habit_completions_habit_date ON habit_completions (habit_id, date);
CREATE INDEX habit_completions_date ON habit_completions (date);
