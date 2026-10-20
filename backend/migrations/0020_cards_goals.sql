-- Karteikarten direkt in Kairo (ohne Notiz) und Fächer mit Farbe. Das Fach einer Karte aus einer Notiz
-- ist ihr erster Ordner; ein Fach mit gleichem Namen fasst beide zusammen.
CREATE TABLE cards (
    id         TEXT PRIMARY KEY,
    deck       TEXT NOT NULL,
    question   TEXT NOT NULL,
    answer     TEXT NOT NULL,
    created_at TEXT NOT NULL
);
CREATE TABLE card_decks (
    name  TEXT PRIMARY KEY,
    color TEXT NOT NULL
);
-- Teilziele: parent_id zeigt auf das Langzeitziel.
ALTER TABLE goals ADD COLUMN parent_id TEXT REFERENCES goals (id) ON DELETE CASCADE;
