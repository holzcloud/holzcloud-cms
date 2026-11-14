-- +goose Up

-- How a request about personal data was made and how it was answered
-- (Phase 33.2, D-01, D-09).
--
-- CCPA regs § 7101(b) ask for a record of every consumer request: the date of
-- the request (created_at), its nature (kind), the manner in which it was
-- made (manner), the date of the response (handled_at), the nature of the
-- response (outcome) and the basis for a denial (basis). With these three
-- columns the record is complete, in the one table that holds the opt-outs
-- of Phase 33.1 and the data requests of Phase 33.2 alike (33.2 D-09).
--
-- manner is 'form' for what the website's own forms took, and the other
-- values for a request the operator enters by hand because it arrived by
-- e-mail, letter or telephone. outcome is empty while nobody has answered.
-- The kind stays checked in Go (internal/dsr), as 00064 explains; manner and
-- outcome are checked here because their lists are the record's vocabulary
-- and do not grow with the program.
--
-- Nothing here deletes a row. Retention decides when one may go (plan 04 of
-- the phase), and 00065 keeps a website with a recent request from being
-- deleted.
ALTER TABLE privacy_requests ADD COLUMN manner TEXT NOT NULL DEFAULT 'form'
    CHECK (manner IN ('form', 'email', 'post', 'phone', 'other'));
ALTER TABLE privacy_requests ADD COLUMN outcome TEXT NOT NULL DEFAULT ''
    CHECK (outcome IN ('', 'granted', 'partly', 'refused'));
ALTER TABLE privacy_requests ADD COLUMN basis TEXT NOT NULL DEFAULT ''
    CHECK (length(basis) <= 2000);

-- +goose Down
-- The rows stay; only how they were made and answered is forgotten.
ALTER TABLE privacy_requests DROP COLUMN basis;
ALTER TABLE privacy_requests DROP COLUMN outcome;
ALTER TABLE privacy_requests DROP COLUMN manner;
