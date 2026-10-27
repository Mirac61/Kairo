-- Neue Karten pro Tag je Fach (wie bei Anki); NULL: Standard (service.NewPerDay). Ein Fach nur mit Limit hat color = ''.
ALTER TABLE card_decks ADD COLUMN new_per_day INTEGER;
