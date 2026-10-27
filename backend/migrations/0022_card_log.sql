-- Verlauf der Lernkarten: jede Bewertung mit dem Lernstand davor (JSON, NULL bei einer neuen Karte),
-- damit „Rückgängig“ ihn wiederherstellen kann und Statistik (Aktivität, Trefferquote, oft vergessen) möglich ist.
CREATE TABLE card_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    key         TEXT NOT NULL,
    grade       INTEGER NOT NULL,
    reviewed_at TEXT NOT NULL,
    prev        TEXT
);
CREATE INDEX card_log_reviewed_at ON card_log (reviewed_at);
