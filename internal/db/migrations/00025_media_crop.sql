-- +goose Up

-- Wie ein Bild zugeschnitten ist.
--
-- Gespeichert wird die Entscheidung, nicht ihr Ergebnis: aus diesen Zahlen und
-- der beiseite gelegten Kopie des Uploads lässt sich der Zuschnitt jederzeit
-- neu berechnen. Wer die Form ändert, beginnt wieder beim Original, statt einen
-- Zuschnitt auf den vorigen zu setzen — sonst verlöre ein Bild bei jeder
-- Meinungsänderung an Qualität.
ALTER TABLE media ADD COLUMN crop_ratio    TEXT    NOT NULL DEFAULT '';
ALTER TABLE media ADD COLUMN crop_zoom     INTEGER NOT NULL DEFAULT 100;
ALTER TABLE media ADD COLUMN crop_rotation INTEGER NOT NULL DEFAULT 0;

-- Wo das Motiv ist, in Prozent von links oben.
--
-- Zwei Aufgaben, dieselbe Zahl. Beim Zuschneiden entscheidet sie, welcher
-- Ausschnitt genommen wird. Beim Ausliefern entscheidet sie, welcher Teil
-- sichtbar bleibt, wenn ein Theme ein Bild in eine feste Form presst — eine
-- Galeriekachel etwa. Ohne sie schneidet der Browser stur aus der Mitte, und
-- bei einem Tier am linken Bildrand ist das jedes Mal daneben.
--
-- 50/50 als Vorgabe: das ist genau das Verhalten von vorher.
ALTER TABLE media ADD COLUMN focus_x INTEGER NOT NULL DEFAULT 50;
ALTER TABLE media ADD COLUMN focus_y INTEGER NOT NULL DEFAULT 50;

-- +goose Down
ALTER TABLE media DROP COLUMN focus_y;
ALTER TABLE media DROP COLUMN focus_x;
ALTER TABLE media DROP COLUMN crop_rotation;
ALTER TABLE media DROP COLUMN crop_zoom;
ALTER TABLE media DROP COLUMN crop_ratio;
