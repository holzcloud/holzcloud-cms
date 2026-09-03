-- +goose Up

-- The basket.
--
-- A row per visitor rather than a server session: someone browsing a shop has
-- no account and no reason to be given one, and a session store that grows by
-- one entry per passing crawler is a cost with nothing behind it. The token is
-- a random value in a cookie; guessing one is as hard as guessing a session id,
-- and losing it costs a basket, not an identity.
CREATE TABLE carts (
    id INTEGER PRIMARY KEY,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    token TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

-- Abandoned baskets are swept by age; without the index that sweep is a scan.
CREATE INDEX idx_carts_updated ON carts (updated_at);

-- What is in it.
--
-- Only the product and a quantity: no price. A basket shows the price the
-- product has now, so an item that got cheaper overnight is cheaper when the
-- visitor comes back — and one that got dearer says so before it is paid for.
-- Prices are frozen at the order, not before, which is where the promise is
-- actually made.
CREATE TABLE cart_items (
    cart_id INTEGER NOT NULL REFERENCES carts(id) ON DELETE CASCADE,
    product_id INTEGER NOT NULL REFERENCES products(id) ON DELETE CASCADE,
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    added_at TEXT NOT NULL,
    PRIMARY KEY (cart_id, product_id)
) STRICT, WITHOUT ROWID;

-- An order, once placed.
--
-- Everything here is a snapshot. A product that is later renamed, repriced or
-- deleted must not change what was ordered — an invoice that rewrites itself is
-- not an invoice. That is why the customer's address, every amount and even the
-- tax rate are stored on the order rather than joined in.
CREATE TABLE orders (
    id INTEGER PRIMARY KEY,
    website_id INTEGER NOT NULL REFERENCES websites(id) ON DELETE CASCADE,
    -- Human-facing and unique per installation: "2026-0007".
    number TEXT NOT NULL UNIQUE,

    -- Which prices this order was quoted in, because the two are computed
    -- differently and the totals below only add up under one of them.
    audience TEXT NOT NULL CHECK (audience IN ('private', 'business')),
    currency TEXT NOT NULL DEFAULT 'CHF',

    email TEXT NOT NULL,
    name TEXT NOT NULL,
    company TEXT NOT NULL DEFAULT '',
    -- The UID of a business customer, as they typed it.
    vat_number TEXT NOT NULL DEFAULT '',
    phone TEXT NOT NULL DEFAULT '',

    street TEXT NOT NULL,
    postal_code TEXT NOT NULL,
    city TEXT NOT NULL,
    country TEXT NOT NULL DEFAULT 'CH',
    note TEXT NOT NULL DEFAULT '',

    -- All amounts in the currency's smallest unit. Items and shipping are kept
    -- apart because an invoice has to show them apart.
    items_net INTEGER NOT NULL,
    items_tax INTEGER NOT NULL,
    items_gross INTEGER NOT NULL,
    shipping_net INTEGER NOT NULL DEFAULT 0,
    shipping_tax INTEGER NOT NULL DEFAULT 0,
    shipping_gross INTEGER NOT NULL DEFAULT 0,
    total_net INTEGER NOT NULL,
    total_tax INTEGER NOT NULL,
    total_gross INTEGER NOT NULL,
    -- Whether tax was charged at all, so an order placed while the shop was
    -- below the threshold keeps saying so after it crosses it.
    vat_exempt INTEGER NOT NULL DEFAULT 0 CHECK (vat_exempt IN (0, 1)),

    status TEXT NOT NULL DEFAULT 'new'
        CHECK (status IN ('new', 'paid', 'shipped', 'cancelled')),
    -- 'invoice' and 'prepay' need no third party. 'payrexx' is the redirect.
    payment_method TEXT NOT NULL DEFAULT 'invoice',
    payment_status TEXT NOT NULL DEFAULT 'open'
        CHECK (payment_status IN ('open', 'paid', 'failed', 'refunded')),
    -- The provider's own identifier, for reconciling a payment later.
    payment_reference TEXT NOT NULL DEFAULT '',

    -- What the shop promised about returns when the order was placed. Copied
    -- rather than joined: Switzerland grants no statutory right of withdrawal,
    -- so this is a voluntary promise, and a promise that changes retroactively
    -- is not one.
    return_policy TEXT NOT NULL DEFAULT '',

    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
) STRICT;

CREATE INDEX idx_orders_website ON orders (website_id, created_at DESC);

-- One line of an order, frozen.
--
-- product_id is nullable and set to NULL when the product is deleted: the line
-- keeps its title, its number and its price, because what was sold does not
-- stop having been sold.
CREATE TABLE order_items (
    id INTEGER PRIMARY KEY,
    order_id INTEGER NOT NULL REFERENCES orders(id) ON DELETE CASCADE,
    product_id INTEGER REFERENCES products(id) ON DELETE SET NULL,

    title TEXT NOT NULL,
    subtitle TEXT NOT NULL DEFAULT '',
    sku TEXT NOT NULL DEFAULT '',
    quantity INTEGER NOT NULL CHECK (quantity > 0),
    tax_bp INTEGER NOT NULL,

    unit_gross INTEGER NOT NULL,
    unit_net INTEGER NOT NULL,
    line_net INTEGER NOT NULL,
    line_tax INTEGER NOT NULL,
    line_gross INTEGER NOT NULL,
    position INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE INDEX idx_order_items_order ON order_items (order_id, position);

-- +goose Down
DROP INDEX IF EXISTS idx_order_items_order;
DROP TABLE IF EXISTS order_items;
DROP INDEX IF EXISTS idx_orders_website;
DROP TABLE IF EXISTS orders;
DROP TABLE IF EXISTS cart_items;
DROP INDEX IF EXISTS idx_carts_updated;
DROP TABLE IF EXISTS carts;
