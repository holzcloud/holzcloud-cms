-- +goose Up

-- Mit welcher Identität beim Ausweisdienst ein Zugang verknüpft ist.
--
-- Phase 10 hat eine Identität dem Zugang mit derselben E-Mail-Adresse
-- zugeordnet. Eine Adresse ist aber nur etwas, das die Identität behauptet:
-- wer den Ausweisdienst eine gewählte Adresse ausgeben lassen kann, wurde zu
-- dem Zugang, der sie trägt — auch zu einem Administrator. Die Roadmap hatte
-- das Gegenteil verlangt, den Benutzernamen; umgesetzt wurde die Adresse, weil
-- das users-Schema für diese Phase unverändert bleiben sollte und die Adresse
-- sein einziger eindeutiger Schlüssel war.
--
-- Leer heisst: mit keiner Identität verknüpft, und damit über die Anmeldung
-- beim Ausweisdienst gar nicht erreichbar. Ein automatisch angelegter Zugang
-- wird in derselben Funktion verknüpft, die ihn anlegt; einen von Hand
-- angelegten verknüpft der Betreiber ausdrücklich.
--
-- Verglichen wird genau, ohne Faltung: jede Faltung wäre eine zweite
-- Definition von "dieselbe Identität" neben der des Ausweisdienstes.
ALTER TABLE users ADD COLUMN sso_username TEXT;

-- Eine Identität, ein Zugang. Der Index gilt nur für verknüpfte Zugänge, denn
-- beliebig viele sind mit keiner verknüpft.
CREATE UNIQUE INDEX idx_users_sso_username ON users(sso_username)
WHERE sso_username IS NOT NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_users_sso_username;
ALTER TABLE users DROP COLUMN sso_username;
