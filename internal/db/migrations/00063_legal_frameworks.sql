-- +goose Up

-- The legal frameworks a website follows.
--
-- LEG-01 of phase 33.1. The operator says per website which bodies of law
-- apply — EU (GDPR and ePrivacy), CH (the revised FADP), UK (UK GDPR and PECR),
-- US (the state privacy laws) — and the privacy page, the imprint and the
-- signals a visitor's browser can send follow from that choice.
--
-- One canonical comma-separated string, like extra_locales, rather than four
-- columns, because the set is what the operator thinks in: "EU and
-- Switzerland", not four separate switches.
--
-- The default writes EU + CH into every website that exists today, and every
-- website created later starts with it too. That is not a guess: the privacy
-- template of Phase 30 was written for exactly those two, so a website that
-- has been running on it is already following them (D-05, confirmed by the
-- owner).
--
-- The CHECK and legal.ParseSet enforce the same sixteen spellings twice, as
-- site_values does: the Go side normalises what anybody typed, and the
-- database refuses whatever slips past it. The empty string is one of them on
-- purpose. A website with no framework is allowed (D-16); the launch checklist
-- reports it instead of the store refusing it.
ALTER TABLE websites ADD COLUMN legal_frameworks TEXT NOT NULL DEFAULT 'eu,ch'
    CHECK (legal_frameworks IN ('', 'eu', 'ch', 'uk', 'us',
                                'eu,ch', 'eu,uk', 'eu,us', 'ch,uk', 'ch,us', 'uk,us',
                                'eu,ch,uk', 'eu,ch,us', 'eu,uk,us', 'ch,uk,us',
                                'eu,ch,uk,us'));

-- +goose Down
-- Dropping the column forgets which frameworks each website chose. Nothing
-- else is lost: the pages written from that choice are ordinary pages and stay
-- as they are.
ALTER TABLE websites DROP COLUMN legal_frameworks;
