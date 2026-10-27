-- Symbol einer Gewohnheit (Name aus dem Icon-Satz des Frontends, leer = automatisch).
ALTER TABLE habits ADD COLUMN icon TEXT NOT NULL DEFAULT '';
