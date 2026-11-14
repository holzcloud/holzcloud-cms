package privacy

import (
	"strconv"
	"strings"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/legal"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The privacy page: one page per website with a shared core and one section
// per chosen legal framework (D-06).
//
// The core says what the program does with personal data on this website,
// derived from the functions it uses (D-02): the request log, the cookies of
// the register, orders, payment, protected pages, forms, the accounts of the
// people who work on it, and its pictures and files. It names no legal basis,
// because a legal basis belongs to one legal order. Each framework's section
// says what that order requires: the bases or justifications, the rights, the
// authority, the transfer rules and its cookie rule.

func init() {
	has := func(f legal.Framework) func(Request) bool {
		return func(r Request) bool { return r.Set.Has(f) }
	}
	register(kindSpec{
		kind:       KindPrivacy,
		minFillins: 2,
		vars:       privacyVars,
		blocks: []block{
			{name: "glance"},
			{name: "controller"},
			{name: "log"},
			{name: "cookies"},
			{name: "orders", on: func(r Request) bool { return r.Features.Shop }},
			{name: "payment", on: func(r Request) bool { return r.Features.Shop && r.Features.Payment }},
			{name: "protected", on: func(r Request) bool { return r.Features.ProtectedPages }},
			{name: "forms", on: func(r Request) bool { return r.Features.Forms }},
			{name: "staff"},
			{name: "media"},
			{name: "eu", on: has(legal.EU)},
			{name: "ch", on: has(legal.CH)},
			{name: "uk", on: has(legal.UK)},
			{name: "us", on: has(legal.US)},
			{name: "closing"},
		},
	})
}

// ComposePrivacy writes the Markdown of a privacy page. The same request always
// gives the same text.
//
// The text is a template: it opens by saying it is not finished, marks every
// fact the operator must supply with the language's fill-in marker, and closes
// by saying it is no legal advice. It is rendered through page.RenderMarkdown
// like any page.
func ComposePrivacy(r Request) string { return compose(KindPrivacy, r) }

// privacyVars are the placeholders of the privacy page.
//
// The numbers of the log section are the constants of package web, which a
// test ties to the journald drop-in in deploy/. The two tables and the lists
// of each framework are built here from rows in meta.json, so their shape is
// the same in every language and only the words differ.
func privacyVars(lang string, r Request) map[string]string {
	m := func(key string) string { return meta(KindPrivacy, lang, key) }
	f := r.Features
	cookies := Cookies(f)

	vars := map[string]string{
		"ipv4": strconv.Itoa(web.LogIPv4Prefix),
		"ipv6": strconv.Itoa(web.LogIPv6Prefix),
		"days": strconv.Itoa(web.LogRetentionDays),
	}

	if len(cookies) == 0 {
		vars["cookie_table"] = m("no_cookies")
	} else {
		var b strings.Builder
		b.WriteString(m("cookies_some") + "\n\n")
		b.WriteString(m("cookie_header") + "\n| --- | --- | --- | --- |")
		for _, c := range cookies {
			b.WriteString("\n| `" + c.Name + "` | " + m(purposeKey(c.Trigger)) + " | " + lifetimeText(lang, c.Lifetime) + " | " + m("why_no_consent") + " |")
		}
		vars["cookie_table"] = b.String()
	}

	processors := m("processor_header") + "\n| --- | --- | --- | --- |\n" + m("processor_hosting")
	if f.Payment {
		processors += "\n" + m("processor_payment")
	}
	vars["processor_rows"] = processors

	// The bases of each framework follow the same functions as the core, so a
	// framework's list never names a processing the website does not do.
	for _, fw := range []string{"eu", "ch", "uk"} {
		rows := []string{m("basis_" + fw + "_log")}
		if len(cookies) > 0 {
			rows = append(rows, m("basis_"+fw+"_cookies"))
		}
		if f.Shop {
			rows = append(rows, m("basis_"+fw+"_orders"))
		}
		if f.Payment {
			rows = append(rows, m("basis_"+fw+"_payment"))
		}
		if f.Forms {
			rows = append(rows, m("basis_"+fw+"_forms"))
		}
		rows = append(rows, m("basis_"+fw+"_staff"))
		vars["bases_"+fw] = bullets(rows)
	}

	// The identifiers are always collected: the opt-out form and the accounts
	// of the people who work on the website take a name and an e-mail address.
	cats := []string{m("category_us_identifiers")}
	if f.Shop {
		cats = append(cats, m("category_us_commercial"))
	}
	cats = append(cats, m("category_us_internet"))
	vars["categories_us"] = bullets(cats)

	return vars
}

func bullets(rows []string) string {
	return "- " + strings.Join(rows, "\n- ")
}

// purposeKey is the meta key of the purpose cell of a cookie. It begins with
// what the visitor does to cause the cookie.
func purposeKey(t Trigger) string {
	switch t {
	case TriggerBasket:
		return "purpose_basket"
	case TriggerPriceView:
		return "purpose_price_view"
	case TriggerUnlock:
		return "purpose_unlock"
	}
	return ""
}

// lifetimeText writes a duration the way a visitor reads it, in the words of
// the text language: whole days, else whole hours, else minutes.
func lifetimeText(lang string, d time.Duration) string {
	const day = 24 * time.Hour
	var unit string
	var n int
	switch {
	case d >= day && d%day == 0:
		unit, n = "day", int(d/day)
	case d >= time.Hour && d%time.Hour == 0:
		unit, n = "hour", int(d/time.Hour)
	default:
		unit, n = "minute", int(d/time.Minute)
	}
	key := "lifetime_" + unit + "_many"
	if n == 1 {
		key = "lifetime_" + unit + "_one"
	}
	return strings.ReplaceAll(meta(KindPrivacy, lang, key), "{n}", strconv.Itoa(n))
}
