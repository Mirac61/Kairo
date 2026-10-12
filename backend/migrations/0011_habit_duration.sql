-- Dauer eines Habits in Minuten, für den Block im Kalender (zusammen mit preferred_time).
ALTER TABLE habits ADD COLUMN duration_minutes INTEGER NOT NULL DEFAULT 30 CHECK (duration_minutes > 0);
