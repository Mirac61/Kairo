-- Priorität eines Ziels (etwa einer Klausur), Werte wie bei Tasks.
ALTER TABLE goals ADD COLUMN priority TEXT NOT NULL DEFAULT 'MEDIUM';
