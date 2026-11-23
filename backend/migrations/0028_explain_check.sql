-- Erklär-Check: Ein Termin mit explain_check erzeugt am Tag jeder Vorlesung eine Task „5 Min erklären: …“.
ALTER TABLE calendar_events ADD COLUMN explain_check INTEGER NOT NULL DEFAULT 0;

-- Herkunft einer vom System erzeugten Task, z. B. explain:<event_id>@<YYYY-MM-DD>. Eindeutig auch im
-- Papierkorb: Eine gelöschte erzeugte Task entsteht nicht neu.
ALTER TABLE tasks ADD COLUMN source_key TEXT;
CREATE UNIQUE INDEX tasks_source_key ON tasks (source_key) WHERE source_key IS NOT NULL;
