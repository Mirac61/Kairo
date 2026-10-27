-- Erinnerung zur Wunschzeit, solange die Gewohnheit offen ist.
ALTER TABLE habits ADD COLUMN remind INTEGER NOT NULL DEFAULT 0 CHECK (remind IN (0, 1));

-- Pausen und ausgesetzte Tage: an diesen Tagen ist nichts fällig, Serien brechen nicht.
-- habit_id NULL = alle Gewohnheiten (Urlaub).
CREATE TABLE habit_pauses (
    id         TEXT PRIMARY KEY,
    habit_id   TEXT REFERENCES habits (id) ON DELETE CASCADE,
    start_date TEXT NOT NULL,
    end_date   TEXT NOT NULL,
    CHECK (end_date >= start_date)
);
