-- Grabsteine endgültig gelöschter erzeugter Tasks: Ihre source_key bleibt hier, damit z. B. ein gelöschter
-- Erklär-Check am selben Tag nicht neu entsteht.
CREATE TABLE deleted_source_keys (source_key TEXT PRIMARY KEY) WITHOUT ROWID;
