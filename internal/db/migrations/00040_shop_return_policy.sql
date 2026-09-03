-- +goose Up

-- What the shop promises about sending goods back.
--
-- Its own migration rather than an edit to 00021, which has already been
-- applied: goose tracks a migration by its version, so changing an applied file
-- adds the column nowhere and leaves a database that fails at the first query
-- with an error that points at the wrong place entirely.
--
-- Switzerland has no statutory right of withdrawal for orders placed online.
-- OR Art. 40a covers doorstep and telephone sales, not a web shop, so anything
-- promised here is a business decision — commonly 14 or 30 days, and lawfully
-- nothing at all, which is the normal case for goods made to measure.
--
-- Free text, because the promise differs by trade and by product, and because a
-- number of days alone would not say what happens to the return postage.
-- Empty is a valid answer: nothing is promised, and the order confirmation says
-- so plainly rather than leaving the customer to assume.
ALTER TABLE websites ADD COLUMN return_policy TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE websites DROP COLUMN return_policy;
