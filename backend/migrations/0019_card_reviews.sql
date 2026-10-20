-- Lernstand der Karteikarten. Die Karten selbst stehen als „Frage :: Antwort“ in den Notizen;
-- key ist ein Hash der Frage, damit der Stand Umbenennen und Verschieben der Notiz übersteht.
CREATE TABLE card_reviews (
    key           TEXT PRIMARY KEY,
    due           TEXT NOT NULL,
    interval_days INTEGER NOT NULL,
    ease          REAL NOT NULL,
    reps          INTEGER NOT NULL,
    reviewed_at   TEXT NOT NULL
);
