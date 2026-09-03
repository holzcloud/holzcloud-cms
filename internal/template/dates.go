package template

import (
	"fmt"
	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"strings"
	"time"
	// tzdata is embedded rather than read from the host. With CGO_ENABLED=0 the
	// binary has no access to the system zoneinfo database, so every
	// LoadLocation would fail and dates would silently fall back to UTC — an
	// hour or two off, all year, with nothing in the logs.
	_ "time/tzdata"
)

// DefaultLocale is what a website gets when nothing else is configured. This is
// a German CMS; English is the fallback for anyone who wants it, not the norm.
const DefaultLocale = "de"

// DefaultTimeZone matches the migration default.
const DefaultTimeZone = "Europe/Berlin"

// localeNames holds the month and weekday names for the supported locales.
//
// Hand-written rather than pulled from golang.org/x/text: two languages need
// about thirty lines, while the CLDR tables behind the plural and gender
// machinery would add megabytes to a binary that runs on an SD card.
type localeNames struct {
	months   [12]string
	weekdays [7]string
	// long is the fmt layout for a written-out date, given day, month name, year.
	long string
	// short is a time layout for the numeric form.
	short string
}

var locales = map[string]localeNames{
	"de": {
		months: [12]string{"Januar", "Februar", "März", "April", "Mai", "Juni",
			"Juli", "August", "September", "Oktober", "November", "Dezember"},
		weekdays: [7]string{"Sonntag", "Montag", "Dienstag", "Mittwoch",
			"Donnerstag", "Freitag", "Samstag"},
		long:  "%d. %s %d",
		short: "02.01.2006",
	},
	"en": {
		months: [12]string{"January", "February", "March", "April", "May", "June",
			"July", "August", "September", "October", "November", "December"},
		weekdays: [7]string{"Sunday", "Monday", "Tuesday", "Wednesday",
			"Thursday", "Friday", "Saturday"},
		long:  "%[2]s %[1]d, %[3]d",
		short: "01/02/2006",
	},
}

// KnownLocale reports whether a locale has date names. Used by the settings
// form so an unknown value cannot be stored and then render as English.
func KnownLocale(locale string) bool {
	_, ok := locales[normalizeLocale(locale)]
	return ok
}

// SupportedLocales lists the locales the settings form offers.
var SupportedLocales = []struct {
	Code string
	Name string
}{
	// The names are marked for translation and translated where they are
	// rendered, like every other language name in the administration: an
	// English administration should not offer "Deutsch" beside "German".
	{Code: "de", Name: i18n.N("Deutsch")},
	{Code: "en", Name: i18n.N("Englisch")},
}

func normalizeLocale(locale string) string {
	locale = strings.ToLower(strings.TrimSpace(locale))
	// Accept "de-DE" and "de_AT" as German; the names are identical.
	if i := strings.IndexAny(locale, "-_"); i > 0 {
		locale = locale[:i]
	}
	if locale == "" {
		return DefaultLocale
	}
	return locale
}

func namesFor(locale string) localeNames {
	if n, ok := locales[normalizeLocale(locale)]; ok {
		return n
	}
	return locales[DefaultLocale]
}

// DateText writes one date the way this website's theme writes dates.
//
// Exported for the places outside a template that put a date on a page — a
// date field inside a block, whose HTML is produced when the page is saved. The
// alternative would be a second date format living somewhere else, and two
// dates spelled differently on one page is the kind of detail a reader notices
// without being able to say what is wrong.
func DateText(locale, timezone string, t time.Time) string {
	return newDateFormatter(locale, timezone).formatLong(&t)
}

// LoadLocation resolves an IANA zone name, falling back to the default and then
// to UTC. A misconfigured zone must not take the site down.
func LoadLocation(name string) *time.Location {
	if name == "" {
		name = DefaultTimeZone
	}
	if loc, err := time.LoadLocation(name); err == nil {
		return loc
	}
	if loc, err := time.LoadLocation(DefaultTimeZone); err == nil {
		return loc
	}
	return time.UTC
}

// dateFormatter builds the date helpers for one website's locale and zone.
//
// Stored timestamps are UTC; rendering them without converting would put a post
// published at 00:30 local time on the previous day.
type dateFormatter struct {
	names localeNames
	loc   *time.Location
}

func newDateFormatter(locale, timezone string) dateFormatter {
	return dateFormatter{names: namesFor(locale), loc: LoadLocation(timezone)}
}

// long renders "2. April 2026" / "April 2, 2026".
func (f dateFormatter) formatLong(t *time.Time) string {
	if t == nil {
		return ""
	}
	local := t.In(f.loc)
	return fmt.Sprintf(f.names.long, local.Day(), f.names.months[int(local.Month())-1], local.Year())
}

// short renders the numeric form, "02.04.2026" / "04/02/2026".
func (f dateFormatter) formatShort(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(f.loc).Format(f.names.short)
}

// iso renders the machine-readable form for <time datetime="…">.
func (f dateFormatter) formatISO(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.In(f.loc).Format("2006-01-02")
}

// weekday renders the day name, for themes that show one.
func (f dateFormatter) formatWeekday(t *time.Time) string {
	if t == nil {
		return ""
	}
	return f.names.weekdays[int(t.In(f.loc).Weekday())]
}
