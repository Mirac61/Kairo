-- Wiederkehrende Tasks: RRULE-Teilmenge wie bei Terminen (FREQ=DAILY|WEEKLY, INTERVAL, BYDAY, UNTIL).
-- Beim Erledigen entsteht die nächste Task, die Regel wandert mit.
ALTER TABLE tasks ADD COLUMN recurrence_rule TEXT;
