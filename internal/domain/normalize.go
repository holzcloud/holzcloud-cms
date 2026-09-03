package domain

import (
	"errors"
	"fmt"
	"strings"

	"golang.org/x/net/idna"
)

// maxDomainLength is the DNS limit for a fully qualified name.
const maxDomainLength = 253

// NormalizeDomain brings a user-entered hostname into the exact form the
// resolver compares the Host header against, and rejects anything that could
// never match.
//
// The resolver looks up stripPort(r.Host), so a stored value carrying a scheme,
// a port, a path or a trailing dot would silently never resolve — the site would
// simply be unreachable with no error anywhere. Normalising on write turns that
// into an immediate, visible validation failure.
func NormalizeDomain(raw string) (string, error) {
	d := strings.ToLower(strings.TrimSpace(raw))
	if d == "" {
		return "", errors.New("domain must not be empty")
	}

	// Tolerate a pasted URL: strip scheme, then anything from the first slash.
	for _, scheme := range []string{"http://", "https://"} {
		d = strings.TrimPrefix(d, scheme)
	}
	if i := strings.IndexAny(d, "/?#"); i >= 0 {
		d = d[:i]
	}
	// Strip credentials ("user@host") and a trailing port.
	if i := strings.LastIndex(d, "@"); i >= 0 {
		d = d[i+1:]
	}
	if i := strings.LastIndex(d, ":"); i >= 0 {
		d = d[:i]
	}
	// A fully qualified name may end in a dot; the Host header will not.
	d = strings.TrimSuffix(d, ".")

	if d == "" {
		return "", errors.New("domain must not be empty")
	}
	if len(d) > maxDomainLength {
		return "", errors.New("domain is too long")
	}

	// Browsers send internationalised names in their punycode form, so "möbel.de"
	// has to be stored as "xn--mbel-5qa.de" or it could never match a Host header.
	ascii, err := idna.Lookup.ToASCII(d)
	if err != nil {
		return "", fmt.Errorf("not a valid domain name: %w", err)
	}
	d = ascii

	if len(d) > maxDomainLength {
		return "", errors.New("domain is too long")
	}

	// "localhost" and similar single labels are legitimate for a home server.
	for _, label := range strings.Split(d, ".") {
		if err := validateLabel(label); err != nil {
			return "", err
		}
	}
	return d, nil
}

// DisplayDomain converts a stored punycode name back to its readable Unicode
// form for the admin UI. Names that are already ASCII come back unchanged, and
// anything that fails to convert is returned as-is rather than hidden.
func DisplayDomain(stored string) string {
	unicode, err := idna.Lookup.ToUnicode(stored)
	if err != nil {
		return stored
	}
	return unicode
}

// validateLabel checks one dot-separated part of a hostname against the
// letter-digit-hyphen rule.
func validateLabel(label string) error {
	if label == "" {
		return errors.New("domain must not contain an empty label")
	}
	if len(label) > 63 {
		return errors.New("domain label is longer than 63 characters")
	}
	if strings.HasPrefix(label, "-") || strings.HasSuffix(label, "-") {
		return errors.New("domain label must not start or end with a hyphen")
	}
	for _, c := range label {
		if !((c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') || c == '-') {
			return errors.New("domain may only contain letters, digits, hyphens and dots")
		}
	}
	return nil
}
