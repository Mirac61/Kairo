-- Papierkorb für Tasks, Termine und Habits: NULL = aktiv, sonst der Zeitpunkt des Löschens.
-- Teilaufgaben einer gelöschten Task tragen denselben Zeitstempel (Wiederherstellen holt sie zusammen zurück).
-- Projekte werden weiterhin hart gelöscht.
ALTER TABLE tasks ADD COLUMN deleted_at TEXT;
ALTER TABLE calendar_events ADD COLUMN deleted_at TEXT;
ALTER TABLE habits ADD COLUMN deleted_at TEXT;
