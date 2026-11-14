-- +goose Up

-- Finding a person by address, per website (Phase 33.2, DSR-01, D-01).
--
-- The Privacy screen asks every store of one website for the rows that carry
-- one e-mail address, whatever its letter case. People type their address the
-- way they like, and "Ada@Example.com" in an order is the same person as
-- "ada@example.com" in a request.
--
-- The statements read `WHERE website_id = ? AND email = ? COLLATE NOCASE`.
-- That shape can use an index declared with the same collation; the obvious
-- `lower(email) = lower(?)` could not use any index and would read the whole
-- table for every search (Pitfall 8 of the phase research).
--
-- mail_outbox gets none: it is small, sent rows are pruned after a week, and
-- it is scanned per website. users.email is already UNIQUE COLLATE NOCASE.
CREATE INDEX orders_by_email ON orders (website_id, email COLLATE NOCASE);
CREATE INDEX form_messages_by_email ON form_messages (website_id, email COLLATE NOCASE);
CREATE INDEX outbox_by_recipient ON outbox (website_id, recipient COLLATE NOCASE);
CREATE INDEX privacy_requests_by_email ON privacy_requests (website_id, email COLLATE NOCASE);

-- +goose Down
DROP INDEX IF EXISTS privacy_requests_by_email;
DROP INDEX IF EXISTS outbox_by_recipient;
DROP INDEX IF EXISTS form_messages_by_email;
DROP INDEX IF EXISTS orders_by_email;
