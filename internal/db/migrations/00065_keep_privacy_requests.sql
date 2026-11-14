-- +goose Up

-- A website that received a privacy request in the last 24 months is not
-- deleted.
--
-- WR-06 of the 33.1 review. privacy_requests.website_id is ON DELETE CASCADE
-- (00064), so deleting a website deleted its requests with it, and with them
-- the record CCPA regs § 7101 ask a business to keep for at least 24 months.
-- internal/dsr deletes nothing for exactly that reason (D-14); the website's
-- deletion was the way around it.
--
-- A trigger and not a check in Go, because a website is deleted from three
-- places (the settings screen, the assistant's delete_website, and whatever
-- comes next) and the rule has to hold for all of them. internal/domain turns
-- the abort into ErrWebsiteKeepsRequests; its message is matched there, so it
-- is not to be reworded here alone.
--
-- 24 months from the later of arrival and answer: § 7101 asks for the record
-- of the request and of the response. A request past that floor goes with its
-- website, as before — Phase 33.2's retention may delete such a row anyway,
-- and an operator who deletes the whole website has decided as much.
--
-- Not "keep the rows detached": a request without a website would be readable
-- by no screen, belong to no website, and break the rule that every statement
-- on this table is keyed by one.

-- +goose StatementBegin
CREATE TRIGGER websites_keep_privacy_requests
BEFORE DELETE ON websites
FOR EACH ROW
WHEN EXISTS (
    SELECT 1 FROM privacy_requests
    WHERE website_id = OLD.id
      AND (created_at >= strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '-24 months')
           OR handled_at >= strftime('%Y-%m-%dT%H:%M:%SZ', 'now', '-24 months'))
)
BEGIN
    SELECT RAISE(ABORT, 'website keeps privacy requests younger than 24 months');
END;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS websites_keep_privacy_requests;
