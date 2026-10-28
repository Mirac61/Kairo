-- Wann ein Ziel erreicht wurde, für den Verlauf im Diagramm; ältere erreichte Ziele bleiben ohne Datum.
ALTER TABLE goals ADD COLUMN archived_at TEXT;
