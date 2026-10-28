-- Symbol eines Projekts (Name aus dem Icon-Satz des Frontends, leer = automatisch aus dem Namen geraten).
ALTER TABLE projects ADD COLUMN icon TEXT NOT NULL DEFAULT '';
