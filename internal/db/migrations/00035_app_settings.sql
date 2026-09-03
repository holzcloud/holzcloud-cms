-- +goose Up

-- Einstellungen der Anlage selbst, nicht einer Website.
--
-- Bisher gab es dafür keinen Ort: eine Website hat ihre Einstellungen, eine
-- Person ihre Sprache, und alles, was für die ganze Installation gilt, stand
-- in Umgebungsvariablen. Das ist richtig für Ports und Passwörter und falsch
-- für alles, was jemand im Browser ändern können soll, ohne einen Dienst neu
-- zu starten.
--
-- Ein Schlüssel-Wert-Paar und keine Spaltenliste, weil hier über die Jahre
-- Kleinigkeiten dazukommen: der Name der Anlage heute, etwas anderes morgen.
-- Für alles Grössere gibt es eine eigene Tabelle.
CREATE TABLE app_settings (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
) STRICT;

-- +goose Down
DROP TABLE IF EXISTS app_settings;
