-- Gewohnheit mit Projekt: Sie gilt als erledigt, sobald an einem Tag duration_minutes im Projekt erfasst sind.
ALTER TABLE habits ADD COLUMN project_id TEXT REFERENCES projects (id) ON DELETE SET NULL;
