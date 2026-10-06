-- +goose Up

-- An unguessable handle for the order confirmation page.
--
-- The page used to live at /bestellung/2026-0001, and the number counts up. Anyone
-- could walk through every order of a shop and read names, addresses and what
-- was bought. The number stays what the customer reads and quotes; the link now
-- carries 128 random bits instead.
ALTER TABLE orders ADD COLUMN token TEXT NOT NULL DEFAULT '';
-- randomblob is evaluated once per row, so every existing order gets its own.
UPDATE orders SET token = lower(hex(randomblob(16))) WHERE token = '';
CREATE UNIQUE INDEX orders_token_idx ON orders(token);

-- +goose Down
DROP INDEX IF EXISTS orders_token_idx;
ALTER TABLE orders DROP COLUMN token;
