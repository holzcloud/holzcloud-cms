package main

// The privacy hook (Phase 33.2, D-09) for the farm shop's orders.
//
// An order carries the person's name, address, e-mail and telephone, and only
// this plugin knows how it is stored. Erasing one is a reduction, not a
// delete: the received order is the basis of the invoice the operator writes
// outside the CMS, and stays that until the retention period ends (D-06,
// D-07). The customer's note stays with it — the owner's rule for an order
// keeps the note — and only e-mail and telephone go.

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/mail"
	"sort"
	"strconv"
	"strings"
	"time"

	plugin "github.com/holzcloud/holzcloud-cms/sdk"
)

func init() { plugin.OnPrivacy(privacyAnswer) }

// listBound is the most keys the host lists in one call.
const listBound = 1000

// maxExpire bounds one expire call, whatever the host asks for: a delete is
// a host call, and the whole call has the two seconds of every hook.
const maxExpire = 200

// keptFields are what a reduced order keeps, by their stable names.
var keptFields = []string{"name", "address", "items", "sum", "note"}

func privacyAnswer(in plugin.PrivacyIn) (plugin.PrivacyOut, error) {
	switch in.Op {
	case plugin.PrivacyFind:
		return privacyFind(in.Email)
	case plugin.PrivacyErase:
		return privacyErase(in.Keys, in.RetainUntil)
	case plugin.PrivacyCorrect:
		return privacyCorrect(in)
	case plugin.PrivacyExpire:
		return privacyExpire(in)
	}
	return plugin.PrivacyOut{}, fmt.Errorf("unknown privacy operation %q", in.Op)
}

// ownOrder reports whether a key is an order. The settings and the hourly
// counters live in the same store and are nobody's personal data.
func ownOrder(key string) bool {
	return strings.HasPrefix(key, prefixOrder) && len(key) > len(prefixOrder)
}

// privacyFind returns the orders placed with the address, in any letter
// case. A reduced order has no address left and is therefore never found by
// it.
func privacyFind(email string) (plugin.PrivacyOut, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		// An empty address would match every reduced order.
		return plugin.PrivacyOut{}, errors.New("find needs an address")
	}
	var out plugin.PrivacyOut
	err := walk(prefixOrder, time.Now().UTC(), func(key, raw string) (bool, error) {
		var b order
		if json.Unmarshal([]byte(raw), &b) != nil {
			return false, nil
		}
		if strings.EqualFold(strings.TrimSpace(b.Email), email) {
			out.Records = append(out.Records, privacyRecord(key, b))
		}
		return false, nil
	})
	if err != nil {
		return plugin.PrivacyOut{}, err
	}
	return out, nil
}

// privacyRecord is an order as the Privacy screen shows it, under the labels
// of the plugin's own screen.
func privacyRecord(key string, b order) plugin.PrivacyRecord {
	rec := plugin.PrivacyRecord{
		Key:    key,
		Kind:   plugin.PrivacyKindOrder,
		Label:  plugin.T("Order"),
		Time:   b.Eingegangen,
		Erased: b.ErasedAt != "",
	}
	if !rec.Erased {
		rec.Correctable = []string{"name", "email", "phone", "address"}
	}
	add := func(name, label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		rec.Fields = append(rec.Fields, plugin.PrivacyField{Name: name, Label: label, Value: value})
	}
	add("name", plugin.T("Name"), b.Name)
	add("email", plugin.T("E-mail"), b.Email)
	add("phone", plugin.T("Telephone"), b.Telefon)
	add("address", plugin.T("Address"), b.Address)
	add("note", plugin.T("Remark"), b.Bemerkung)
	add("page", plugin.T("Ordered on"), b.Page)
	for i, p := range b.Posten {
		line := zahl(p.Quantity) + " × " + p.Titel
		if p.Price != "" {
			line += " – " + b.Currency + " " + p.Price
			if p.Einheit != "" {
				line += " / " + p.Einheit
			}
		}
		add("item_"+strconv.Itoa(i+1), plugin.T("Item"), line)
	}
	add("sum", plugin.T("Total"), summeText(b))
	add("currency", plugin.T("Currency"), b.Currency)
	// Times as stored, RFC 3339, under a name ending in _at: the Privacy
	// screen shows such a field on the website's clock, which this plugin
	// does not know.
	add("received_at", plugin.T("Received"), b.Eingegangen)
	if b.Done {
		add("done", plugin.T("Status"), plugin.T("done"))
	}
	if rec.Erased {
		add("erased_at", plugin.T("Deleted on"), b.ErasedAt)
	}
	return rec
}

// privacyErase reduces the orders under keys to what the invoice needs and
// says what it kept and until when.
//
// Without retainUntil there is nothing true to tell the person, so nothing
// is erased; and every key is checked before the first order changes.
func privacyErase(keys []string, retainUntil string) (plugin.PrivacyOut, error) {
	if len(keys) == 0 {
		return plugin.PrivacyOut{}, errors.New("erase needs at least one key")
	}
	until, err := time.Parse(time.RFC3339, retainUntil)
	if err != nil {
		return plugin.PrivacyOut{}, errors.New("erase needs retain_until, an RFC 3339 time: an order is kept until then")
	}
	for _, key := range keys {
		if !ownOrder(key) {
			return plugin.PrivacyOut{}, fmt.Errorf("%q is not an order of this plugin", key)
		}
	}
	now := time.Now().UTC().Format(time.RFC3339)
	var out plugin.PrivacyOut
	for _, key := range keys {
		raw, ok, err := plugin.Get(key)
		if err != nil {
			return plugin.PrivacyOut{}, err
		}
		if !ok {
			continue
		}
		var b order
		if err := json.Unmarshal([]byte(raw), &b); err != nil {
			return plugin.PrivacyOut{}, fmt.Errorf("the order %q cannot be read: %w", key, err)
		}
		b.Email, b.Telefon = "", ""
		if b.ErasedAt == "" {
			b.ErasedAt = now
		}
		// Erased twice, the later date holds: what the person was told the
		// first time must not become untrue.
		keep := until.UTC()
		if earlier, err := time.Parse(time.RFC3339, b.RetainUntil); err == nil && earlier.After(keep) {
			keep = earlier
		}
		b.RetainUntil = keep.Format(time.RFC3339)
		data, err := json.Marshal(b)
		if err != nil {
			return plugin.PrivacyOut{}, err
		}
		if err := plugin.Set(key, string(data)); err != nil {
			return plugin.PrivacyOut{}, err
		}
		out.Kept = append(out.Kept, plugin.PrivacyKept{Key: key, Fields: keptFields, Until: b.RetainUntil})
	}
	plugin.Logf("info", "privacy: %d orders reduced to the invoice basis", len(out.Kept))
	return out, nil
}

// privacyCorrect sets one of the person's details on a live order. A reduced
// order is refused: it is an invoice basis now, and what it says is what the
// operator invoiced.
func privacyCorrect(in plugin.PrivacyIn) (plugin.PrivacyOut, error) {
	if !ownOrder(in.Key) {
		return plugin.PrivacyOut{}, fmt.Errorf("%q is not an order of this plugin", in.Key)
	}
	value := strings.TrimSpace(in.Value)
	switch in.Field {
	case "name":
		if value == "" || len([]rune(value)) > maxName {
			return plugin.PrivacyOut{}, fmt.Errorf("a name has to be between 1 and %d characters", maxName)
		}
	case "email":
		if !bareAddress(value) {
			return plugin.PrivacyOut{}, errors.New("the new value is not one bare e-mail address")
		}
	case "phone":
		if len([]rune(value)) > maxTelefon {
			return plugin.PrivacyOut{}, fmt.Errorf("a telephone number has at most %d characters", maxTelefon)
		}
	case "address":
		if len([]rune(value)) > maxAddress {
			return plugin.PrivacyOut{}, fmt.Errorf("an address has at most %d characters", maxAddress)
		}
	default:
		return plugin.PrivacyOut{}, fmt.Errorf("the field %q cannot be corrected", in.Field)
	}

	raw, ok, err := plugin.Get(in.Key)
	if err != nil {
		return plugin.PrivacyOut{}, err
	}
	if !ok {
		return plugin.PrivacyOut{}, fmt.Errorf("there is no order under %q", in.Key)
	}
	var b order
	if err := json.Unmarshal([]byte(raw), &b); err != nil {
		return plugin.PrivacyOut{}, err
	}
	if b.ErasedAt != "" {
		return plugin.PrivacyOut{}, errors.New("the order was erased and is kept as an invoice basis; it is not corrected")
	}
	switch in.Field {
	case "name":
		b.Name = value
	case "email":
		b.Email = value
	case "phone":
		b.Telefon = value
	case "address":
		b.Address = value
	}
	// Under the key it was read from, not one built from the id: the host
	// named this key, and it is the one find will return again.
	data, err := json.Marshal(b)
	if err != nil {
		return plugin.PrivacyOut{}, err
	}
	if err := plugin.Set(in.Key, string(data)); err != nil {
		return plugin.PrivacyOut{}, err
	}
	return plugin.PrivacyOut{Records: []plugin.PrivacyRecord{privacyRecord(in.Key, b)}}, nil
}

// bareAddress reports whether s is exactly one address with nothing around
// it: no display name, no second address, no angle brackets.
func bareAddress(s string) bool {
	if s == "" || !addressLooksReal(s) {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Name == "" && a.Address == s
}

// privacyExpire deletes the orders that came in before the cut-off, oldest
// first, at most the limit per call — except an erased order whose stored
// date has not come yet. The host computes the cut-off from today's period;
// the stored date is what the person was told, and the later of the two
// holds.
func privacyExpire(in plugin.PrivacyIn) (plugin.PrivacyOut, error) {
	if in.Kind != plugin.PrivacyKindOrder {
		return plugin.PrivacyOut{}, nil
	}
	before, err := time.Parse(time.RFC3339, in.Before)
	if err != nil {
		return plugin.PrivacyOut{}, fmt.Errorf("the cut-off %q is not an RFC 3339 time", in.Before)
	}
	limit := in.Limit
	if limit <= 0 || limit > maxExpire {
		limit = maxExpire
	}
	now := time.Now().UTC()

	var out plugin.PrivacyOut
	err = walk(prefixOrder, before, func(key, raw string) (bool, error) {
		var b order
		_ = json.Unmarshal([]byte(raw), &b)
		at, ok := orderTime(b, strings.TrimPrefix(key, prefixOrder))
		if !ok {
			return false, nil
		}
		if !at.Before(before) {
			// Keys sort by time: everything after this is later still.
			return true, nil
		}
		if b.RetainUntil != "" {
			if keep, err := time.Parse(time.RFC3339, b.RetainUntil); err != nil || keep.After(now) {
				// Kept until a date that has not come, or one that cannot be
				// read: left alone either way.
				return false, nil
			}
		}
		if out.Deleted >= limit {
			out.More = true
			return true, nil
		}
		if err := plugin.Delete(key); err != nil {
			return true, err
		}
		out.Deleted++
		return false, nil
	})
	if err != nil {
		return plugin.PrivacyOut{}, err
	}
	plugin.Logf("info", "privacy: %d orders past the retention period removed", out.Deleted)
	return out, nil
}

// orderTime is when an order came in: its stored time, or the time its key
// begins with.
func orderTime(b order, rest string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, b.Eingegangen); err == nil {
		return t, true
	}
	if len(rest) >= 15 {
		if t, err := time.Parse("20060102T150405", rest[:15]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// walk calls f for every key under prefix, oldest first, until f says stop
// or the keys run past the year of until.
//
// The host lists at most listBound keys per call and has no offset, so the
// store is read a year at a time — every key begins with its time — and a
// year that fills a whole call is read again a month at a time. A month that
// still fills one is an error naming the month: an answer that silently
// stopped at a thousand orders would look complete.
func walk(prefix string, until time.Time, f func(key, raw string) (stop bool, err error)) error {
	first, err := plugin.List(prefix, 1)
	if err != nil {
		return err
	}
	var oldest string
	for k := range first {
		oldest = k
	}
	if oldest == "" {
		return nil
	}
	start, ok := keyYear(strings.TrimPrefix(oldest, prefix))
	if !ok {
		return fmt.Errorf("a key under %q does not begin with a date", prefix)
	}
	// One year beyond, for a clock that ran ahead when an order came in.
	end := until.Year() + 1
	for year := start; year <= end; year++ {
		batch, err := plugin.List(fmt.Sprintf("%s%04d", prefix, year), listBound)
		if err != nil {
			return err
		}
		if len(batch) >= listBound {
			for month := 1; month <= 12; month++ {
				part, err := plugin.List(fmt.Sprintf("%s%04d%02d", prefix, year, month), listBound)
				if err != nil {
					return err
				}
				if len(part) >= listBound {
					return fmt.Errorf("%04d-%02d holds at least %d entries under %q, more than one call can list", year, month, listBound, prefix)
				}
				if stop, err := each(part, f); stop || err != nil {
					return err
				}
			}
			continue
		}
		if stop, err := each(batch, f); stop || err != nil {
			return err
		}
	}
	return nil
}

// each calls f for the keys of one listing in key order.
func each(batch map[string]string, f func(key, raw string) (bool, error)) (bool, error) {
	keys := make([]string, 0, len(batch))
	for k := range batch {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		stop, err := f(k, batch[k])
		if stop || err != nil {
			return stop, err
		}
	}
	return false, nil
}

// keyYear reads the year a key begins with.
func keyYear(rest string) (int, bool) {
	if len(rest) < 4 {
		return 0, false
	}
	y, err := strconv.Atoi(rest[:4])
	return y, err == nil && y > 0
}
