-- Projektfarbe (Name aus der Palette in domain.ProjectColors). Bestehende Projekte bekommen
-- reihum je eine Farbe nach Anlegedatum, damit sie sich unterscheiden.
ALTER TABLE projects ADD COLUMN color TEXT NOT NULL DEFAULT 'violet';
UPDATE projects SET color = CASE (
    SELECT COUNT(*) FROM projects p
    WHERE p.created_at < projects.created_at OR (p.created_at = projects.created_at AND p.id < projects.id)
) % 8
    WHEN 0 THEN 'violet' WHEN 1 THEN 'blue' WHEN 2 THEN 'orange' WHEN 3 THEN 'aqua'
    WHEN 4 THEN 'pink' WHEN 5 THEN 'yellow' WHEN 6 THEN 'green' ELSE 'red'
END;
