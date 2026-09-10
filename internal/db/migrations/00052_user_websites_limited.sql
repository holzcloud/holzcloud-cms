-- +goose Up

-- Ob jemand auf seine Websites begrenzt ist, steht ab hier ausdrücklich da.
--
-- 00033 hat die Grenze allein aus den Zeilen in user_websites gelesen: keine
-- Zeile hiess alle Websites. Das war für die Einführung richtig — sonst wäre
-- die Migration selbst eine Aussperrung gewesen —, und es hat eine Tür offen
-- gelassen, durch die man nicht hinein-, sondern hinausgeht: user_websites
-- verliert Zeilen, ohne dass jemand den Menschen anfasst. Wird die einzige
-- Website eines Redakteurs gelöscht, räumt ON DELETE CASCADE seine einzige
-- Zeile weg, und aus "nur diese eine" wird "alle". Dasselbe, wenn SetRights
-- zwischen DELETE und INSERT scheitert, und wenn die Anmeldung über Authentik
-- einen Administrator herabstuft, dem keine Website-Gruppe bleibt.
--
-- Die Spalte trennt die beiden Bedeutungen, die bisher dieselbe Form hatten:
-- nie begrenzt (0) und begrenzt auf das, was dasteht — auch wenn nichts
-- mehr dasteht (1).
ALTER TABLE users ADD COLUMN websites_limited INTEGER NOT NULL DEFAULT 0;

-- Genau die Zugänge, die heute eine Zeile haben, sind heute begrenzt. Danach
-- erreicht jeder dieselben Websites wie vorher: niemand verliert etwas, und
-- niemand gewinnt etwas.
UPDATE users SET websites_limited = 1
WHERE id IN (SELECT user_id FROM user_websites);

-- +goose Down
-- Zurück heisst zurück zur alten Lesart: ein begrenzter Zugang ohne Zeile
-- erreicht danach wieder jede Website. Das ist der Fehler, den Up behebt.
ALTER TABLE users DROP COLUMN websites_limited;
