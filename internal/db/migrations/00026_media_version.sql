-- +goose Up

-- Wie oft die ausgelieferte Datei sich geändert hat.
--
-- Bis zum Zuschneiden war eine Medien-Adresse unveränderlich: was einmal unter
-- /media/1/weide.jpg lag, lag dort für immer, und genau das sagt der Server dem
-- Browser mit "immutable" und einem Jahr Haltbarkeit.
--
-- Zuschneiden bricht dieses Versprechen — dieselbe Adresse liefert danach
-- andere Bytes. Ohne Gegenmassnahme zeigt jeder Browser, der das Bild schon
-- einmal geladen hat, bis zu ein Jahr lang das alte. Beim Bauen ist das nicht
-- zu sehen; es fällt erst dem Betreiber auf, dessen Zuschnitt scheinbar nichts
-- bewirkt hat.
--
-- Die Zahl hängt beim Ausliefern als ?v= an der Adresse. Eine neue Zahl ist
-- eine neue Adresse, und damit stimmt "immutable" wieder.
ALTER TABLE media ADD COLUMN version INTEGER NOT NULL DEFAULT 0;

-- +goose Down
ALTER TABLE media DROP COLUMN version;
