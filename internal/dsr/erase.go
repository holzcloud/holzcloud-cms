package dsr

// Erasure: one person's records on one website go, or are reduced to what the
// law asks to keep (Phase 33.2, DSR-03, D-03, D-06, D-07, D-08, D-09).
//
// How an order is erased was decided by the phase's research and then by the
// owner (D-03, D-07, confirmed 2026-11-14), not assumed here:
//
//   - An order is the seller's copy of an invoice. UStG § 14b asks for the
//     copy ("Doppel der Rechnung") to be kept, and AO § 146(4) forbids
//     changing a record "in einer Weise …, dass der ursprüngliche Inhalt
//     nicht mehr feststellbar ist". So the fields the invoice prints — name,
//     company, VAT number, address, items, amounts, payment state, return
//     policy and the customer's note — are never rewritten, blanked or
//     "anonymised" by an erasure. GDPR Art. 17(3)(b) ("to the extent that
//     processing is necessary … for compliance with a legal obligation") is
//     what allows keeping exactly those and nothing else.
//   - What the invoice does not print goes: the e-mail address and the phone
//     number are emptied, and the order's mails — which repeat address and
//     items — are deleted.
//   - The confirmation link's token is replaced by a fresh random value of
//     the same shape, not emptied: orders_token_idx is UNIQUE, and a second
//     erased order with an empty token would break it (research Pitfall 7).
//     The old link answers 404 from then on.
//   - The period follows the seller's country (D-06, legal.OrderPeriodFor),
//     never shorter than the operator's own order period (Service.OrderYears),
//     counted from the end of the calendar year of the order
//     (legal.OrderDeletableFrom). The date is stored with the order as
//     retain_until: what the person was told stays true when the period
//     changes later, and the retention job honours it (expireOrders never
//     deletes an order before its stored date).
//
// The keys of what is erased come from a search made by Erase itself, on the
// website it is given, never from the request: whatever a form posts, it can
// only name the address, and the address only reaches that website's records.
//
// Backups and pre-upgrade snapshots are not touched (D-08); the admin screen
// says which ones still hold the data and when they age out.

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
)

// The actions an erasure takes on a record.
const (
	ActionDelete = "delete"
	ActionReduce = "reduce"
	ActionKeep   = "keep"
)

// The source groups an operator ticks on the erasure page.
const (
	EraseOrders       = "orders"
	EraseFormMessages = "form_messages"
	EraseMail         = "mail"
	ErasePlugins      = "plugins"
	EraseAccounts     = "accounts"
)

// EraseOptions is what one erasure is asked to do.
type EraseOptions struct {
	// Email is the person's address; the records are found by it again.
	Email string
	// Sources are the groups to erase, by the Erase* names. A group that is
	// not ticked is left as it is.
	Sources map[string]bool
	// Accounts allows the staff account to be erased. Only an administrator
	// may; the caller decides who that is.
	Accounts bool
	// ActorUserID and ActorEmail are the staff member who erases, for the
	// activity log and so that nobody erases their own account from here.
	ActorUserID int64
	ActorEmail  string
	// RequestID is the privacy request the erasure answers, 0 for none.
	RequestID int64
	// Now is the moment of the erasure; zero means the clock.
	Now time.Time
}

// ErasureItem is one record as the erasure treats it.
type ErasureItem struct {
	// Source and Key name the record as Record does.
	Source, Key string
	// Label names the record, a catalogue key for the core sources.
	Label string
	// Action is ActionDelete, ActionReduce or ActionKeep.
	Action string
	// Gone and Kept are what goes and what stays of a reduced record, as
	// catalogue keys for the core sources and the plugin's own labels for a
	// plugin.
	Gone, Kept []string
	// Reason says why the record is reduced or kept, a catalogue key.
	Reason string
	// Basis names the law of the period, a catalogue key, "" when none.
	Basis string
	// Until is the date the record is deleted, nil when it goes now.
	Until *time.Time
}

// ErasurePreview is what an erasure would do, before it does anything.
type ErasurePreview struct {
	Items []ErasureItem
	// Holders are the plugins that keep data for the website, for their
	// names.
	Holders []Holder
	// NotCovered are the plugins that keep data and cannot be asked: what
	// they keep is not erased.
	NotCovered []Holder
	// Failed are the stores that could not be searched.
	Failed []SourceError
}

// ErasureReport is what an erasure did.
type ErasureReport struct {
	// Deleted counts the records that went, by source group.
	Deleted map[string]int
	// Reduced are the records reduced to what the law asks to keep.
	Reduced []ErasureItem
	// Kept are the records kept whole, with the reason.
	Kept []ErasureItem
	// FileErrors name the files that stayed on the disk although their
	// record went.
	FileErrors []string
	// Failed are the stores or records that could not be erased.
	Failed []SourceError
	// NotCovered are the plugins that keep data and were not asked.
	NotCovered []Holder
	// Holders are the plugins that keep data for the website, for their
	// names.
	Holders []Holder
	// RequestAnswered is true when the erasure recorded the answer of the
	// request it was made for.
	RequestAnswered bool
}

// The labels of what an order loses and keeps, catalogue keys, in the order
// the screens list them.
var (
	orderGone = []string{
		i18n.N("E-mail address"),
		i18n.N("Phone"),
		i18n.N("Confirmation link"),
		i18n.N("Order mails"),
	}
	orderKept = []string{
		i18n.N("Name"),
		i18n.N("Company"),
		i18n.N("VAT number"),
		i18n.N("Address"),
		i18n.N("Items"),
		i18n.N("Amounts"),
		i18n.N("Payment status"),
		i18n.N("Promised returns policy"),
		i18n.N("Remark"),
	}
	// orderReduced is the reason an order is reduced and not deleted.
	orderReduced = i18n.N("An order's invoice record must stay readable until its period ends, so it is reduced, not deleted: name, company, address, items, amounts, payment state, return policy and the customer's note stay; the e-mail address and the phone number go.")
)

// erasePlan is the search an erasure works from, with what the dispositions
// need from the website.
type erasePlan struct {
	res        *Result
	orderYears int
	orderLaw   legal.OrderPeriod
}

// plan searches the website for the address and reads the periods.
func (s *Service) plan(ctx context.Context, websiteID int64, country, email string) (*erasePlan, error) {
	res, err := s.Find(ctx, websiteID, email)
	if err != nil {
		return nil, err
	}
	years, law, err := s.OrderYears(ctx, websiteID, country)
	if err != nil {
		return nil, err
	}
	return &erasePlan{res: res, orderYears: years, orderLaw: law}, nil
}

// orderUntil is the date an order placed at created is deleted.
func (p *erasePlan) orderUntil(created time.Time) *time.Time {
	until := legal.OrderDeletableFrom(created, p.orderYears)
	return &until
}

// disposition is what the erasure does with one record. ok is false for a
// record no erasure touches.
func (p *erasePlan) disposition(rec Record) (ErasureItem, bool) {
	it := ErasureItem{Source: rec.Source, Key: rec.Key, Label: rec.Label}
	switch rec.Source {
	case EraseOrders:
		it.Action = ActionReduce
		it.Gone, it.Kept = orderGone, orderKept
		it.Reason, it.Basis = orderReduced, p.orderLaw.Basis()
		it.Until = p.orderUntil(rec.Time)
		return it, true
	}
	return it, false
}

// Preview is what erasing the address on the website would do. It changes
// nothing.
func (s *Service) Preview(ctx context.Context, websiteID int64, country, email string, now time.Time) (*ErasurePreview, error) {
	p, err := s.plan(ctx, websiteID, country, email)
	if err != nil {
		return nil, err
	}
	out := &ErasurePreview{NotCovered: p.res.NotCovered(), Holders: p.res.Holders, Failed: p.res.Failed}
	for _, rec := range p.res.Records {
		if it, ok := p.disposition(rec); ok {
			out.Items = append(out.Items, it)
		}
	}
	return out, nil
}

// Erase erases the address on the website: it searches again, and erases
// what the search finds in the ticked groups. A record that cannot be erased
// is reported in Failed and the rest goes on; the error is for what stops the
// whole erasure, such as an address that is not one (ErrBadEmail).
func (s *Service) Erase(ctx context.Context, websiteID int64, country string, opt EraseOptions) (*ErasureReport, error) {
	now := opt.Now
	if now.IsZero() {
		now = time.Now()
	}
	now = now.UTC()
	p, err := s.plan(ctx, websiteID, country, opt.Email)
	if err != nil {
		return nil, err
	}
	rep := &ErasureReport{Deleted: map[string]int{}, NotCovered: p.res.NotCovered(), Holders: p.res.Holders,
		Failed: p.res.Failed}
	for _, rec := range p.res.Records {
		it, ok := p.disposition(rec)
		if !ok {
			continue
		}
		switch rec.Source {
		case EraseOrders:
			if !opt.Sources[EraseOrders] {
				continue
			}
			done, err := s.reduceOrder(ctx, websiteID, rec.Key, opt.Email, now, it.Until)
			switch {
			case err != nil:
				rep.fail(websiteID, rec.Source, err)
			case done:
				rep.Reduced = append(rep.Reduced, it)
			}
		}
	}
	return rep, nil
}

// reduceOrder reduces one order to its invoice record (D-07), in one write
// transaction with the deletion of its mails. done is false when the order
// is no longer there to reduce — erased in the meantime, or its address
// changed — which is not an error.
//
// The order is read again by number, website and address inside the
// transaction: the number came from a search a moment ago, and between the
// two somebody may have corrected the address.
func (s *Service) reduceOrder(ctx context.Context, websiteID int64, number, email string, now time.Time, until *time.Time) (bool, error) {
	tx, err := s.db.Write.BeginTx(ctx, nil)
	if err != nil {
		return false, fmt.Errorf("order: %w", err)
	}
	defer tx.Rollback()
	var id int64
	err = tx.QueryRowContext(ctx, `SELECT id FROM orders WHERE website_id = $1 AND number = $2
		AND email = $3 COLLATE NOCASE AND erased_at IS NULL`, websiteID, number, email).Scan(&id)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	if err != nil {
		return false, fmt.Errorf("order: %w", err)
	}
	stamp := now.Format(timeLayout)
	if _, err := tx.ExecContext(ctx, `UPDATE orders SET email = '', phone = '',
		token = lower(hex(randomblob(16))), erased_at = $3, retain_until = $4, updated_at = $3
		WHERE website_id = $1 AND id = $2 AND erased_at IS NULL`,
		websiteID, id, stamp, until.UTC().Format(timeLayout)); err != nil {
		return false, fmt.Errorf("order: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `DELETE FROM outbox WHERE website_id = $1 AND order_id = $2`,
		websiteID, id); err != nil {
		return false, fmt.Errorf("order mails: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return false, fmt.Errorf("order: %w", err)
	}
	return true, nil
}

// fail records a source or record that could not be erased, and writes it
// to the journal with the website and source only: the error of a plugin is
// the plugin's free text, and an address must not reach the journal.
func (r *ErasureReport) fail(websiteID int64, source string, err error) {
	r.Failed = append(r.Failed, SourceError{Source: source, Err: err})
	slog.Warn("privacy erasure: a source could not be erased", "website", websiteID, "source", source,
		"cause", ErasureFailure(err))
}

// The sentences of what could not be erased, catalogue keys.
var (
	eraseFailedDatabase = i18n.N("The database refused the deletion.")
)

// ErasureFailure is the sentence of one failure of an erasure, a catalogue
// key. The error itself is for the journal.
func ErasureFailure(err error) string {
	if err == nil {
		return ""
	}
	return eraseFailedDatabase
}
