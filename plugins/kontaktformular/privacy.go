package main

// The privacy hook (Phase 33.2, D-09): the operator asks which messages
// belong to a person, and later erases, corrects or expires them.
//
// Only this plugin knows how a message is stored, so the host asks rather
// than reads. Three places hold a person's address: a message from the
// contact form, a submission the spam traps turned away (the quarantine), and
// an answer typed into an e-mail field of an assembled form, whose sender
// field may be empty. A search that looked only at the first would report
// "nothing found" with the other two still on the disk.

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

// correctable are the fields correct accepts on every record.
var correctable = []string{"name", "email"}

// maxExpire bounds one expire call, whatever the host asks for: a delete is
// a host call, and the whole call has the two seconds of every hook.
const maxExpire = 200

func privacyAnswer(in plugin.PrivacyIn) (plugin.PrivacyOut, error) {
	switch in.Op {
	case plugin.PrivacyFind:
		return privacyFind(in.Email)
	case plugin.PrivacyErase:
		return privacyErase(in.Keys)
	case plugin.PrivacyCorrect:
		return privacyCorrect(in)
	case plugin.PrivacyExpire:
		return privacyExpire(in)
	}
	return plugin.PrivacyOut{}, fmt.Errorf("unknown privacy operation %q", in.Op)
}

// ownRecord reports whether a key is one of the records this hook answers
// for: a message or a refused submission. The hourly counters, the signing
// key and the operator's settings live in the same store and are nobody's
// personal data.
func ownRecord(key string) bool {
	for _, prefix := range []string{prefixMessage, prefixQuarantine} {
		if strings.HasPrefix(key, prefix) && len(key) > len(prefix) {
			return true
		}
	}
	return false
}

// privacyErase deletes the records under keys and hands back the media ids
// of their attachments, for the host to delete with them.
//
// Every key is checked before the first is deleted: a call that names one
// key outside the records deletes nothing.
func privacyErase(keys []string) (plugin.PrivacyOut, error) {
	if len(keys) == 0 {
		return plugin.PrivacyOut{}, errors.New("erase needs at least one key")
	}
	for _, key := range keys {
		if !ownRecord(key) {
			return plugin.PrivacyOut{}, fmt.Errorf("%q is not a message of this plugin", key)
		}
	}
	var out plugin.PrivacyOut
	for _, key := range keys {
		raw, ok, err := plugin.Get(key)
		if err != nil {
			return plugin.PrivacyOut{}, err
		}
		if !ok {
			// Already gone — deleted by the operator, or by an earlier erase.
			continue
		}
		var n message
		if json.Unmarshal([]byte(raw), &n) == nil {
			for _, a := range n.Attachments {
				out.MediaIDs = append(out.MediaIDs, a.MediaID)
			}
		}
		if err := plugin.Delete(key); err != nil {
			return plugin.PrivacyOut{}, err
		}
		out.Deleted++
	}
	plugin.Logf("info", "privacy: %d messages erased, %d attachments handed back", out.Deleted, len(out.MediaIDs))
	return out, nil
}

// privacyCorrect sets the sender's name or address of one record.
//
// An address that also stands in an answer of an assembled form is replaced
// there too: find matches answers, so an address corrected only in the
// sender field would go on finding the record under the old one. The old
// address is the record's sender address and the address the host searched
// for (PrivacyIn.Email) when it passes one; a record with neither carries
// the person's address only in its answers, and then every answer that is
// an address is the person's.
func privacyCorrect(in plugin.PrivacyIn) (plugin.PrivacyOut, error) {
	if !ownRecord(in.Key) {
		return plugin.PrivacyOut{}, fmt.Errorf("%q is not a message of this plugin", in.Key)
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
	default:
		return plugin.PrivacyOut{}, fmt.Errorf("the field %q cannot be corrected", in.Field)
	}

	raw, ok, err := plugin.Get(in.Key)
	if err != nil {
		return plugin.PrivacyOut{}, err
	}
	if !ok {
		return plugin.PrivacyOut{}, fmt.Errorf("there is no message under %q", in.Key)
	}
	var n message
	if err := json.Unmarshal([]byte(raw), &n); err != nil {
		return plugin.PrivacyOut{}, err
	}

	if in.Field == "name" {
		n.Name = value
	} else {
		var old []string
		for _, a := range []string{n.Email, in.Email} {
			if a = strings.TrimSpace(a); a != "" {
				old = append(old, a)
			}
		}
		if len(old) == 0 {
			for _, a := range n.Fields {
				if v := strings.TrimSpace(a.Value); plausibleAddress(v) {
					old = append(old, v)
				}
			}
		}
		for i, a := range n.Fields {
			for _, o := range old {
				if strings.EqualFold(strings.TrimSpace(a.Value), o) {
					n.Fields[i].Value = value
				}
			}
		}
		n.Email = value
	}

	// Written back the way every write of this plugin is: the same key, the
	// same encoding.
	updated, err := json.Marshal(n)
	if err != nil {
		return plugin.PrivacyOut{}, err
	}
	if err := plugin.Set(in.Key, string(updated)); err != nil {
		return plugin.PrivacyOut{}, err
	}
	return plugin.PrivacyOut{Records: []plugin.PrivacyRecord{privacyRecord(in.Key, n)}}, nil
}

// bareAddress reports whether s is exactly one address with nothing around
// it: no display name, no second address, no angle brackets.
func bareAddress(s string) bool {
	if s == "" || len(s) > maxEmail || !plausibleAddress(s) {
		return false
	}
	a, err := mail.ParseAddress(s)
	return err == nil && a.Name == "" && a.Address == s
}

// privacyExpire deletes the messages and refused submissions that came in
// before the cut-off, oldest first, at most the limit per call.
//
// Read or unread makes no difference here. sweep spares the unread because
// an enquiry nobody has read is one nobody has dealt with; the retention
// period is the operator's own decision about how long anything is kept,
// and a message they never opened is still past it.
func privacyExpire(in plugin.PrivacyIn) (plugin.PrivacyOut, error) {
	if in.Kind != plugin.PrivacyKindFormSubmission {
		// Not a kind this plugin keeps: nothing to do, and no error.
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

	var out plugin.PrivacyOut
	for _, prefix := range []string{prefixMessage, prefixQuarantine} {
		err := walk(prefix, before, func(key, raw string) (bool, error) {
			var n message
			_ = json.Unmarshal([]byte(raw), &n)
			at, ok := entryTime(n, strings.TrimPrefix(key, prefix))
			if !ok {
				// A record whose time cannot be read is left alone rather
				// than guessed old.
				return false, nil
			}
			if !at.Before(before) {
				// Keys sort by time: everything after this is later still.
				return true, nil
			}
			if out.Deleted >= limit {
				out.More = true
				return true, nil
			}
			if err := plugin.Delete(key); err != nil {
				return true, err
			}
			out.Deleted++
			for _, a := range n.Attachments {
				out.MediaIDs = append(out.MediaIDs, a.MediaID)
			}
			return false, nil
		})
		if err != nil {
			return plugin.PrivacyOut{}, err
		}
		if out.More {
			break
		}
	}
	plugin.Logf("info", "privacy: %d messages past the retention period removed", out.Deleted)
	return out, nil
}

// entryTime is when a record came in: its stored time, or the time its key
// begins with.
func entryTime(n message, rest string) (time.Time, bool) {
	if t, err := time.Parse(time.RFC3339, n.Time); err == nil {
		return t, true
	}
	if len(rest) >= 20 {
		if t, err := time.Parse(time.RFC3339, rest[:20]); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// privacyFind returns every message and refused submission that carries the
// address, as sender or as an answer, in any letter case.
func privacyFind(email string) (plugin.PrivacyOut, error) {
	email = strings.TrimSpace(email)
	if email == "" {
		// An empty address would match every message without one.
		return plugin.PrivacyOut{}, errors.New("find needs an address")
	}
	var out plugin.PrivacyOut
	for _, prefix := range []string{prefixMessage, prefixQuarantine} {
		err := walk(prefix, time.Now().UTC(), func(key, raw string) (bool, error) {
			var n message
			if json.Unmarshal([]byte(raw), &n) != nil {
				return false, nil
			}
			if belongsTo(n, email) {
				out.Records = append(out.Records, privacyRecord(key, n))
			}
			return false, nil
		})
		if err != nil {
			return plugin.PrivacyOut{}, err
		}
	}
	return out, nil
}

// belongsTo reports whether the address is the message's sender or one of
// its answers.
func belongsTo(n message, email string) bool {
	if strings.EqualFold(strings.TrimSpace(n.Email), email) {
		return true
	}
	for _, a := range n.Fields {
		if strings.EqualFold(strings.TrimSpace(a.Value), email) {
			return true
		}
	}
	return false
}

// privacyRecord is a stored message as the Privacy screen shows it: every
// value it holds, under the labels of the plugin's own screen.
func privacyRecord(key string, n message) plugin.PrivacyRecord {
	rec := plugin.PrivacyRecord{
		Key:         key,
		Kind:        plugin.PrivacyKindFormSubmission,
		Time:        n.Time,
		Correctable: correctable,
	}
	switch {
	case strings.HasPrefix(key, prefixQuarantine):
		rec.Label = plugin.T("Turned away")
	case n.FormName != "":
		// The operator's own name for the form, not translated.
		rec.Label = n.FormName
	default:
		rec.Label = plugin.T("Form message")
	}

	add := func(name, label, value string) {
		if strings.TrimSpace(value) == "" {
			return
		}
		rec.Fields = append(rec.Fields, plugin.PrivacyField{Name: name, Label: label, Value: value})
	}
	add("name", plugin.T("Name"), n.Name)
	add("email", plugin.T("E-mail"), n.Email)
	add("subject", plugin.T("Subject"), n.Subject)
	add("text", plugin.T("Message"), n.Text)
	add("page", plugin.T("Page"), n.Page)
	add("form", plugin.T("Form"), n.FormName)
	for i, a := range n.Fields {
		// The label is the question the operator wrote.
		add("answer_"+strconv.Itoa(i+1), a.Label, strings.TrimSpace(a.Value))
	}
	if n.Consent != "" {
		add("consent", plugin.T("Consent"), plugin.Tf("Agreed on %s: “%s”", shortDate(n.ConsentTime), n.Consent))
	}
	for i, a := range n.Attachments {
		add("attachment_"+strconv.Itoa(i+1), plugin.T("Attachment"), a.Name)
		rec.MediaIDs = append(rec.MediaIDs, a.MediaID)
	}
	if n.Reply != "" {
		add("reply", plugin.Tf("Answered on %s", shortDate(n.ReplyTime)), n.Reply)
	}
	if n.RefusedFor != "" {
		why := quarantineReasons[n.RefusedFor]
		if why == "" {
			why = n.RefusedFor
		}
		add("refused_for", plugin.T("Reason"), plugin.T(why))
	}
	if n.Read {
		add("read", plugin.T("Status"), plugin.T("Read"))
	}
	return rec
}

// walk calls f for every key under prefix, oldest first, until f says stop
// or the keys run past the year of until.
//
// The host lists at most listBound keys per call and has no offset, so the
// store is read a year at a time — every key begins with its time — and a
// year that fills a whole call is read again a month at a time. A month that
// still fills one is an error naming the month: an answer that silently
// stopped at a thousand messages would look complete.
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
	// One year beyond, for a clock that ran ahead when a message came in.
	end := until.Year() + 1
	for year := start; year <= end; year++ {
		batch, err := plugin.List(fmt.Sprintf("%s%04d", prefix, year), listBound)
		if err != nil {
			return err
		}
		if len(batch) >= listBound {
			batch = nil
			for month := 1; month <= 12; month++ {
				span := fmt.Sprintf("%s%04d-%02d", prefix, year, month)
				part, err := plugin.List(span, listBound)
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
