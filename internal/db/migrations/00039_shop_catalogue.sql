-- +goose Up

-- The catalogue.
--
-- Products are their own table rather than another page kind. A page is text
-- with a title; a product is a price, a stock level, a tax rate and a weight,
-- and every one of those has to be a column a query can filter and sum. Bolting
-- them onto pages would mean a dozen columns that are NULL on every page that
-- is not a product, and a `kind = 'product'` condition on every existing query
-- — including the ones that must never leak a draft.
CREATE TABLE products (
    id INTEGER PRIMARY KEY,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    slug TEXT NOT NULL,
    title TEXT NOT NULL,
    -- A short line under the title: "Eiche massiv, geölt". Not the excerpt,
    -- which is prose; this is the one-line identification.
    subtitle TEXT NOT NULL DEFAULT '',
    description_markdown TEXT NOT NULL DEFAULT '',
    description_html TEXT NOT NULL DEFAULT '',
    excerpt TEXT NOT NULL DEFAULT '',
    -- Artikelnummer.
    sku TEXT NOT NULL DEFAULT '',

    -- Money is in rappen, as an integer. A price in a floating-point number is
    -- a price that will one day be 19.999999 in an invoice total.
    --
    -- The gross price is the stored one and the net is derived from it, not the
    -- other way round. The consumer price is the one the Preisbekanntgabe-
    -- verordnung regulates: it is advertised, it has to be the amount actually
    -- payable including tax, and "49.00" must stay 49.00. Storing net and
    -- multiplying up would produce 52.97 as soon as someone typed a round net
    -- price. Business customers see a net figure derived by division, which can
    -- be an odd number — for them that is normal, because what they care about
    -- is the net sum on the invoice.
    price_gross INTEGER NOT NULL DEFAULT 0,

    -- The tax rate in basis points: 810 is 8.1 %, 260 is 2.6 %, 380 is 3.8 %.
    --
    -- Whole percent would have been enough for Germany and is wrong here:
    -- Switzerland's rates have a decimal place, and every one of the three does.
    -- Basis points keep it an integer, which is what stops it rounding.
    tax_bp INTEGER NOT NULL DEFAULT 810 CHECK (tax_bp >= 0 AND tax_bp <= 10000),

    -- NULL means "not tracked" — a joiner who builds to order has no stock
    -- level, and 0 would wrongly mean sold out.
    stock INTEGER,
    -- Grams. Shipping by weight needs it; 0 means it does not matter.
    weight_grams INTEGER NOT NULL DEFAULT 0,
    -- "Lieferzeit 3–4 Wochen". Free text, because every trade phrases it
    -- differently and a number of days would be a promise nobody can keep.
    delivery_note TEXT NOT NULL DEFAULT '',

    status TEXT NOT NULL DEFAULT 'draft' CHECK (status IN ('draft', 'published')),
    featured_media_id INTEGER REFERENCES media(id) ON DELETE SET NULL,
    position INTEGER NOT NULL DEFAULT 0,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL,

    UNIQUE (website_id, slug)
) STRICT;

-- The public list only ever asks for published products of one website, in
-- order. Without this it is a scan of the whole table on every shop page.
CREATE INDEX idx_products_public
    ON products (website_id, status, position, id);

-- The picture gallery. The featured image is on the product itself because
-- every list needs exactly one and a join for it would be a join per row.
CREATE TABLE product_media (
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    media_id INTEGER NOT NULL REFERENCES media(id) ON DELETE CASCADE,
    position INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY (product_id, position)
) STRICT, WITHOUT ROWID;

-- Categories reuse the existing labels rather than introducing a second,
-- parallel vocabulary. A workshop that labels a post "Eiche" means the same
-- thing when it labels a table "Eiche", and one list of labels is one list to
-- maintain.
CREATE TABLE product_terms (
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    term_id INTEGER NOT NULL REFERENCES terms(id) ON DELETE CASCADE,
    PRIMARY KEY (product_id, term_id)
) STRICT, WITHOUT ROWID;

CREATE INDEX idx_product_terms_term ON product_terms (term_id, product_id);

-- Per-website shop settings.
--
-- shop_base is the path the catalogue lives under, next to blog_base. Empty
-- switches the shop off for that website entirely — a CMS that hosts several
-- sites cannot assume they all sell something.
ALTER TABLE websites ADD COLUMN shop_base TEXT NOT NULL DEFAULT '';

-- The currency, as an ISO 4217 code. Stored per website rather than compiled
-- in: the same installation may run a Swiss and an Austrian site, and the
-- formatting differs by more than the symbol — CHF 1'234.55 against 1.234,55 €.
ALTER TABLE websites ADD COLUMN currency TEXT NOT NULL DEFAULT 'CHF';

-- Shipping as one flat rate with a free-over threshold. A rate table by weight
-- and country is what a bigger shop needs and what a joiner would never fill
-- in; the weight is recorded per product so that table can be added later
-- without touching the products.
ALTER TABLE websites ADD COLUMN shipping_gross INTEGER NOT NULL DEFAULT 0;
-- NULL: no free shipping at any value. 0 would mean "always free".
ALTER TABLE websites ADD COLUMN shipping_free_from INTEGER;
ALTER TABLE websites ADD COLUMN shipping_tax_bp INTEGER NOT NULL DEFAULT 810
    CHECK (shipping_tax_bp >= 0 AND shipping_tax_bp <= 10000);

-- Which price a visitor is shown by default, and whether they may switch.
--
-- 'private'  — gross only, the ordinary consumer shop
-- 'business' — net only, a trade supplier
-- 'both'     — the visitor chooses; the choice lives in a cookie
ALTER TABLE websites ADD COLUMN price_display TEXT NOT NULL DEFAULT 'private'
    CHECK (price_display IN ('private', 'business', 'both'));

-- Below CHF 100'000 of annual turnover a business is not liable for VAT and
-- must not show it. Kept separate from the tax rate so switching it on does not
-- silently rewrite every product's rate — a shop that crosses the threshold
-- switches this off and its products are unchanged.
ALTER TABLE websites ADD COLUMN vat_exempt INTEGER NOT NULL DEFAULT 0
    CHECK (vat_exempt IN (0, 1));

-- The UID, shown on invoices: CHE-123.456.789 MWST.
ALTER TABLE websites ADD COLUMN vat_number TEXT NOT NULL DEFAULT '';

-- +goose Down
ALTER TABLE websites DROP COLUMN vat_number;
ALTER TABLE websites DROP COLUMN vat_exempt;
ALTER TABLE websites DROP COLUMN price_display;
ALTER TABLE websites DROP COLUMN shipping_tax_bp;
ALTER TABLE websites DROP COLUMN shipping_free_from;
ALTER TABLE websites DROP COLUMN shipping_gross;
ALTER TABLE websites DROP COLUMN currency;
ALTER TABLE websites DROP COLUMN shop_base;
DROP INDEX IF EXISTS idx_product_terms_term;
DROP TABLE IF EXISTS product_terms;
DROP TABLE IF EXISTS product_media;
DROP INDEX IF EXISTS idx_products_public;
DROP TABLE IF EXISTS products;
