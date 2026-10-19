-- Einstellungen der Oberfläche (Sprache, Theme, Kalender …) für alle Geräte; eine Zeile mit JSON.
CREATE TABLE ui_prefs (
    id   INTEGER PRIMARY KEY CHECK (id = 1),
    data TEXT NOT NULL
);
