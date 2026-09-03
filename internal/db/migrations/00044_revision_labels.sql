-- +goose Up
-- Eine Beschriftung für eine Fassung. Ein Verlauf aus zwanzig Zeitstempeln
-- beantwortet nicht, welche davon die war, die vor dem Umbau galt — und genau
-- diese sucht man, wenn man den Verlauf überhaupt öffnet.
ALTER TABLE page_revisions ADD COLUMN label TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE page_revisions DROP COLUMN label;
