package dsr

// Erasure: one person's records on one website go, or are reduced to what the
// law asks to keep (Phase 33.2, DSR-03, D-03, D-06, D-07, D-08, D-09).
//
// How an order is erased was decided by the phase's research and then by the
// owner (D-03, D-07, confirmed 2026-11-14), not assumed here:
//
//   - An order is the seller's copy of an invoice. UStG § 14b asks for that
//     copy to be kept, and AO § 146(4) forbids changing a record in a way
//     that leaves its original content no longer ascertainable. So the
//     fields the invoice prints — name,
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
// The other records of the person:
//
//   - Legacy form messages and the person's mail — order mail by recipient,
//     the queue by recipient or Reply-To — are deleted, by id and website.
//   - A plugin erases its own records through the privacy hook: one call for
//     what it deletes (contact-form messages, the quarantine included), one
//     per order with the date it keeps it until. The media ids it returns
//     are claims and go through media.Store.DeleteOwned with this website; a
//     file of another website is refused and reported, a file that stays on
//     the disk is named.
//   - The person's privacy requests stay. They are the record of the request
//     and its answer (CCPA regs § 7101) and what keeps an erasure from being
//     undone by mistake (§ 7022(e)); the retention job deletes them after
//     the request period. This file changes a request only through
//     Store.Answer.
//   - A staff account goes only when the caller says an administrator chose
//     it, never the account of the person erasing, and through the account
//     store, so the last administrator stays. The activity log keeps the
//     staff member's address (D-09, GDPR Art. 17(3)(e)).
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
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/activity"
	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
	"github.com/holzcloud/holzcloud-cms/internal/user"
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
	// pluginOrderReduced is the same for an order a plugin keeps: the plugin
	// says itself what it keeps, and the result page lists it.
	pluginOrderReduced = i18n.N("An order is the basis of an invoice, so the plugin reduces it instead of deleting it: the e-mail address and the phone number go, the rest stays until the period ends.")
	// requestsKept is why the person's own requests stay.
	requestsKept = i18n.N("The record of a request about personal data is kept for its retention period (CCPA regs § 7101) and so that an erasure is not undone by mistake (§ 7022(e)); it is used for nothing else.")
	// eraseAccountsSentence says what erasing a staff account does (D-09,
	// GDPR Art. 17(3)(e)).
	eraseAccountsSentence = i18n.N("The staff account is deleted for the whole installation, with its sessions. The activity log keeps the staff member's address as the record of what they did.")
	// eraseAccountsAdminOnly is why a ticked account stayed.
	eraseAccountsAdminOnly = i18n.N("Only an administrator can erase a staff account.")
	// The answers the erasure records on the request it was made for.
	erasePartlyBasis  = i18n.N("Erased; records the law requires are kept, reduced, until their periods end.")
	eraseGrantedBasis = i18n.N("Erased.")
)

// erasePlan is the search an erasure works from, with what the dispositions
// need from the website.
type erasePlan struct {
	res        *Result
	orderYears int
	orderLaw   legal.OrderPeriod
	// requestMonths is the website's period for privacy requests.
	requestMonths int
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
	periods, err := s.Periods(ctx, websiteID, country)
	if err != nil {
		return nil, err
	}
	p := &erasePlan{res: res, orderYears: years, orderLaw: law, requestMonths: requestFloor}
	for _, per := range periods {
		if per.Kind == RetentionRequest {
			p.requestMonths = per.Months
		}
	}
	return p, nil
}

// orderUntil is the date an order placed at created is deleted.
func (p *erasePlan) orderUntil(created time.Time) *time.Time {
	until := legal.OrderDeletableFrom(created, p.orderYears)
	return &until
}

// requestUntil is the date the retention job deletes a request: its period
// after the later of arrival and answer, as expireRequests counts.
func (p *erasePlan) requestUntil(rec Record) *time.Time {
	from := rec.Time
	for _, f := range rec.Fields {
		if f.Name != "handled_at" || f.Value == "" {
			continue
		}
		if handled := parseTime(f.Value); handled.After(from) {
			from = handled
		}
	}
	until := from.UTC().AddDate(0, p.requestMonths, 0)
	return &until
}

// pluginOrderFields splits the fields of a plugin's order into what an
// erasure removes — the contact fields — and what stays, by their labels.
// Times are not listed: they say when, not what.
func pluginOrderFields(rec Record) (gone, kept []string) {
	seen := map[string]bool{}
	for _, f := range rec.Fields {
		label := f.Label
		if label == "" {
			label = f.Name
		}
		switch {
		case f.Name == "email" || f.Name == "phone":
			gone = append(gone, label)
		case strings.HasSuffix(f.Name, "_at"), seen[label]:
		default:
			seen[label] = true
			kept = append(kept, label)
		}
	}
	return gone, kept
}

// disposition is what the erasure does with one record. ok is false for a
// record no erasure touches: one a plugin has reduced already.
func (p *erasePlan) disposition(rec Record) (ErasureItem, bool) {
	it := ErasureItem{Source: rec.Source, Key: rec.Key, Label: rec.Label, Action: ActionDelete}
	switch {
	case rec.Erased:
		return it, false
	case rec.Source == EraseOrders:
		it.Action = ActionReduce
		it.Gone, it.Kept = orderGone, orderKept
		it.Reason, it.Basis = orderReduced, p.orderLaw.Basis()
		it.Until = p.orderUntil(rec.Time)
	case rec.Source == "requests":
		it.Action = ActionKeep
		it.Reason = requestsKept
		it.Until = p.requestUntil(rec)
	case rec.Source == EraseAccounts:
		it.Reason = eraseAccountsSentence
	case strings.HasPrefix(rec.Source, "plugin:") && rec.Kind == "order":
		it.Action = ActionReduce
		it.Gone, it.Kept = pluginOrderFields(rec)
		it.Reason, it.Basis = pluginOrderReduced, p.orderLaw.Basis()
		it.Until = p.orderUntil(rec.Time)
	}
	return it, true
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
//
// Afterwards it answers the request the erasure was made for, when that is
// an unanswered erasure request of this website, and writes one
// privacy.erase entry to the activity log with the counts — never the
// address.
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
	rep := &ErasureReport{Deleted: map[string]int{}, NotCovered: p.res.NotCovered(), Holders: p.res.Holders}
	for _, f := range p.res.Failed {
		rep.fail(websiteID, f.Source, &eraseError{sentence: eraseFailedNotSearched, detail: "not searched"})
	}

	plugins := map[string][]Record{}
	var pluginOrder []string
	for _, rec := range p.res.Records {
		it, ok := p.disposition(rec)
		if !ok {
			continue
		}
		switch {
		case rec.Source == EraseOrders:
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
		case rec.Source == EraseFormMessages:
			if opt.Sources[EraseFormMessages] {
				s.eraseRow(ctx, websiteID, rec, `DELETE FROM form_messages WHERE website_id = $1 AND id = $2`, rec.Key, rep)
			}
		case rec.Source == EraseMail:
			if !opt.Sources[EraseMail] {
				continue
			}
			if id, ok := strings.CutPrefix(rec.Key, "outbox-"); ok {
				s.eraseRow(ctx, websiteID, rec, `DELETE FROM outbox WHERE website_id = $1 AND id = $2`, id, rep)
			} else if id, ok := strings.CutPrefix(rec.Key, "queue-"); ok {
				s.eraseRow(ctx, websiteID, rec, `DELETE FROM mail_outbox WHERE website_id = $1 AND id = $2`, id, rep)
			}
		case rec.Source == "requests":
			rep.Kept = append(rep.Kept, it)
		case rec.Source == EraseAccounts:
			if !opt.Sources[EraseAccounts] {
				continue
			}
			if !opt.Accounts {
				it.Action, it.Reason = ActionKeep, eraseAccountsAdminOnly
				rep.Kept = append(rep.Kept, it)
				continue
			}
			s.eraseAccount(ctx, websiteID, rec, opt.ActorUserID, rep)
		case strings.HasPrefix(rec.Source, "plugin:"):
			if !opt.Sources[ErasePlugins] {
				continue
			}
			if _, seen := plugins[rec.Source]; !seen {
				pluginOrder = append(pluginOrder, rec.Source)
			}
			plugins[rec.Source] = append(plugins[rec.Source], rec)
		}
	}
	for _, source := range pluginOrder {
		s.erasePlugin(ctx, websiteID, source, plugins[source], p, rep)
	}

	if opt.RequestID > 0 {
		rep.RequestAnswered = s.answerErasure(ctx, websiteID, opt.RequestID, rep)
	}
	s.logErasure(ctx, websiteID, opt, rep)
	return rep, nil
}

// eraseRow deletes one row a search found, by its id and the website, and
// counts it under the record's group when it went.
func (s *Service) eraseRow(ctx context.Context, websiteID int64, rec Record, query, id string, rep *ErasureReport) {
	n, err := strconv.ParseInt(id, 10, 64)
	if err != nil {
		rep.fail(websiteID, rec.Source, fmt.Errorf("%s: a key that is not a row id", rec.Source))
		return
	}
	res, err := s.db.Write.ExecContext(ctx, query, websiteID, n)
	if err != nil {
		rep.fail(websiteID, rec.Source, fmt.Errorf("%s %d: %w", rec.Source, n, err))
		return
	}
	if affected, err := res.RowsAffected(); err == nil && affected > 0 {
		rep.Deleted[rec.Source] += int(affected)
	}
}

// eraseAccount deletes one staff account the operator chose, through the
// account store — whose rules, such as the last administrator staying,
// apply here as everywhere — and ends its sessions. Never the account of
// the person erasing: they would lock themselves out in the middle of it.
// The activity log keeps the address the account acted under (D-09).
func (s *Service) eraseAccount(ctx context.Context, websiteID int64, rec Record, actor int64, rep *ErasureReport) {
	id, err := strconv.ParseInt(rec.Key, 10, 64)
	if err != nil {
		rep.fail(websiteID, rec.Source, fmt.Errorf("account: a key that is not an id"))
		return
	}
	if id == actor {
		rep.fail(websiteID, rec.Source, &eraseError{sentence: eraseFailedOwnAccount, detail: "the account of the person erasing"})
		return
	}
	if s.users == nil {
		rep.fail(websiteID, rec.Source, fmt.Errorf("account %d: no account store", id))
		return
	}
	switch err := s.users.Delete(ctx, id); {
	case errors.Is(err, user.ErrNotFound):
		return
	case errors.Is(err, user.ErrLastAdmin):
		rep.fail(websiteID, rec.Source, &eraseError{sentence: eraseFailedLastAdmin, detail: fmt.Sprintf("account %d is the last administrator", id)})
		return
	case err != nil:
		rep.fail(websiteID, rec.Source, fmt.Errorf("account %d: %w", id, err))
		return
	}
	rep.Deleted[EraseAccounts]++
	if s.sessions != nil {
		if err := auth.DestroyUserSessions(ctx, s.sessions, id, ""); err != nil {
			rep.fail(websiteID, rec.Source, fmt.Errorf("end the sessions of account %d: %w", id, err))
		}
	}
}

// erasePlugin asks one plugin to erase the person's records: one call for
// everything it deletes, and one per order, which it reduces and keeps until
// the date the host computed from the order's own time (D-06, D-07; a
// farm-shop order is an order). The keys are the plugin's own, from the
// search just made.
//
// The plugin's answer is a claim: its media ids go through DeleteOwned with
// this website, and its error is reported by kind, never in its own words.
func (s *Service) erasePlugin(ctx context.Context, websiteID int64, source string, recs []Record, p *erasePlan, rep *ErasureReport) {
	id := strings.TrimPrefix(source, "plugin:")
	byKey := map[string]Record{}
	var keys []string
	var orders []Record
	for _, rec := range recs {
		byKey[rec.Key] = rec
		if rec.Kind == "order" {
			orders = append(orders, rec)
		} else {
			keys = append(keys, rec.Key)
		}
	}
	call := func(in plugin.PrivacyIn) {
		out, err := s.plugins.Privacy(ctx, websiteID, id, in)
		if err != nil {
			rep.fail(websiteID, source, &eraseError{sentence: eraseFailedPlugin, detail: pluginFailure(id, err).Error()})
			return
		}
		rep.Deleted[ErasePlugins] += out.Deleted
		for _, mid := range out.MediaIDs {
			s.eraseMedia(ctx, websiteID, source, mid, rep)
		}
		for _, k := range out.Kept {
			rec, ok := byKey[k.Key]
			if !ok {
				continue
			}
			it, _ := p.disposition(rec)
			it.Action = ActionReduce
			it.Kept = pluginKeptLabels(rec, k.Fields)
			if until, err := time.Parse(time.RFC3339, k.Until); err == nil {
				until = until.UTC()
				it.Until = &until
			}
			rep.Reduced = append(rep.Reduced, it)
		}
	}
	if s.plugins == nil {
		rep.fail(websiteID, source, &eraseError{sentence: eraseFailedPlugin, detail: "plugin " + id + ": no plugin manager"})
		return
	}
	if len(keys) > 0 {
		call(plugin.PrivacyIn{Op: plugin.PrivacyErase, Keys: keys})
	}
	for _, rec := range orders {
		call(plugin.PrivacyIn{Op: plugin.PrivacyErase, Keys: []string{rec.Key},
			RetainUntil: p.orderUntil(rec.Time).Format(time.RFC3339)})
	}
}

// pluginKeptLabels are the labels of the fields a plugin says it kept, read
// from the record as the search returned it. "items" stands for the
// numbered item fields; a name the record does not carry is shown as it is.
func pluginKeptLabels(rec Record, names []string) []string {
	out := make([]string, 0, len(names))
	for _, name := range names {
		label := name
		for _, f := range rec.Fields {
			if f.Name == name || name == "items" && strings.HasPrefix(f.Name, "item_") {
				if f.Label != "" {
					label = f.Label
				}
				break
			}
		}
		out = append(out, label)
	}
	return out
}

// eraseMedia removes one file a plugin named for a record it erased. A file
// of another website is refused and reported; a file that no longer exists
// is what was asked for; a file that stays on the disk is named, with the
// name it has in the media library, read before its row goes.
func (s *Service) eraseMedia(ctx context.Context, websiteID int64, source string, mediaID int64, rep *ErasureReport) {
	var name string
	_ = s.db.Read.QueryRowContext(ctx, `SELECT filename FROM media WHERE id = $1 AND website_id = $2`,
		mediaID, websiteID).Scan(&name)
	if name == "" {
		name = "media " + strconv.FormatInt(mediaID, 10)
	}
	if s.media == nil {
		rep.FileErrors = append(rep.FileErrors, name)
		return
	}
	err := s.media.DeleteOwned(ctx, websiteID, mediaID, s.dataDir)
	switch {
	case err == nil:
	case errors.Is(err, media.ErrNotOwned):
		rep.fail(websiteID, source, &eraseError{sentence: eraseFailedForeignFile,
			detail: fmt.Sprintf("%s named media %d, which is not this website's; it was left alone", source, mediaID)})
	case errors.Is(err, media.ErrNotFound):
		// Gone already, for instance deleted in the media library.
	case errors.Is(err, media.ErrFileRemains):
		rep.FileErrors = append(rep.FileErrors, name)
		slog.Warn("privacy erasure: a file stayed on the disk", "website", websiteID, "media", mediaID)
	default:
		rep.fail(websiteID, source, fmt.Errorf("media %d: %w", mediaID, err))
	}
}

// answerErasure records the answer of the request the erasure was made for:
// only an erasure request of this website that nobody has answered yet. It
// is partly granted when anything was reduced, kept, could not be erased or
// was not searched — the request records themselves do not count, because
// keeping them is what § 7022(e) asks — and granted otherwise. The basis is
// written in the language of the request, as the operator reads it.
func (s *Service) answerErasure(ctx context.Context, websiteID, requestID int64, rep *ErasureReport) bool {
	store := NewStore(s.db)
	r, err := store.Get(ctx, websiteID, requestID)
	if err != nil || r == nil || r.Kind != KindErasure || r.Outcome != "" {
		return false
	}
	partly := len(rep.Reduced) > 0 || len(rep.Failed) > 0 || len(rep.NotCovered) > 0 || len(rep.FileErrors) > 0
	for _, it := range rep.Kept {
		if it.Source != "requests" {
			partly = true
		}
	}
	outcome, basis := OutcomeGranted, eraseGrantedBasis
	if partly {
		outcome, basis = OutcomePartly, erasePartlyBasis
	}
	if err := store.Answer(ctx, websiteID, requestID, outcome, i18n.T(i18n.Lang(ctx), basis)); err != nil {
		if !errors.Is(err, ErrAnswered) {
			slog.Error("privacy erasure: the request could not be answered", "website", websiteID, "request", requestID, "err", err)
		}
		return false
	}
	return true
}

// logErasure writes the one activity entry of an erasure: counts, the
// request, never the address or any value of what went.
func (s *Service) logErasure(ctx context.Context, websiteID int64, opt EraseOptions, rep *ErasureReport) {
	if s.activity == nil {
		return
	}
	meta := map[string]any{
		"deleted":     rep.Deleted,
		"reduced":     len(rep.Reduced),
		"kept":        len(rep.Kept),
		"file_errors": len(rep.FileErrors),
		"failures":    len(rep.Failed),
		"not_covered": len(rep.NotCovered),
	}
	if opt.RequestID > 0 {
		meta["request"] = opt.RequestID
		meta["request_answered"] = rep.RequestAnswered
	}
	var actor *int64
	if opt.ActorUserID != 0 {
		id := opt.ActorUserID
		actor = &id
	}
	ws := websiteID
	s.activity.Log(ctx, activity.Entry{
		UserID:     actor,
		ActorEmail: opt.ActorEmail,
		Action:     activity.ActionPrivacyErase,
		EntityType: "website",
		EntityID:   websiteID,
		WebsiteID:  &ws,
		Metadata:   meta,
	})
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
// to the journal with the website, the source and the fixed sentence only:
// the error of a plugin is the plugin's free text, and an address must not
// reach the journal.
func (r *ErasureReport) fail(websiteID int64, source string, err error) {
	r.Failed = append(r.Failed, SourceError{Source: source, Err: err})
	slog.Warn("privacy erasure: a source could not be erased", "website", websiteID, "source", source,
		"cause", ErasureFailure(err))
}

// eraseError is a failure of an erasure with the sentence the operator
// reads; the detail is for the journal and carries ids only.
type eraseError struct {
	sentence, detail string
}

func (e *eraseError) Error() string { return e.detail }

// The sentences of what could not be erased, catalogue keys.
var (
	eraseFailedDatabase    = i18n.N("The database refused the deletion.")
	eraseFailedNotSearched = i18n.N("This store could not be searched, so nothing of it was erased.")
	eraseFailedPlugin      = i18n.N("The plugin could not erase its records; they are still there.")
	eraseFailedForeignFile = i18n.N("A plugin named a file that is not this website's; it was left alone.")
	eraseFailedOwnAccount  = i18n.N("Your own account cannot be erased from here.")
	eraseFailedLastAdmin   = i18n.N("The last administrator cannot be deleted")
)

// ErasureFailure is the sentence of one failure of an erasure, a catalogue
// key. The error itself is for the journal.
func ErasureFailure(err error) string {
	if err == nil {
		return ""
	}
	var e *eraseError
	if errors.As(err, &e) {
		return e.sentence
	}
	return eraseFailedDatabase
}
