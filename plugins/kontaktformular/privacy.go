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

func privacyAnswer(in plugin.PrivacyIn) (plugin.PrivacyOut, error) {
	switch in.Op {
	case plugin.PrivacyFind:
		return privacyFind(in.Email)
	case plugin.PrivacyErase, plugin.PrivacyCorrect, plugin.PrivacyExpire:
		return plugin.PrivacyOut{}, fmt.Errorf("privacy operation %q is not built yet", in.Op)
	}
	return plugin.PrivacyOut{}, fmt.Errorf("unknown privacy operation %q", in.Op)
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
