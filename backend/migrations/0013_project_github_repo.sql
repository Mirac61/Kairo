-- GitHub-Repo eines Projekts („owner/name“), für Issues pro Projekt, wenn der Server
-- den Projektordner nicht sieht (Pi-Betrieb). NULL: aus git remote von local_path ableiten.
ALTER TABLE projects ADD COLUMN github_repo TEXT;
