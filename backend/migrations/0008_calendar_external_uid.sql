-- UID aus einem ICS-Import; ein erneuter Import aktualisiert den Treffer statt zu duplizieren.
ALTER TABLE calendar_events ADD COLUMN external_uid TEXT;
CREATE UNIQUE INDEX calendar_events_external_uid ON calendar_events (external_uid) WHERE external_uid IS NOT NULL;
