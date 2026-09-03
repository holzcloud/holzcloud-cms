package domain

import "testing"

func TestNormalizeDomain(t *testing.T) {
	accepted := map[string]string{
		"example.com":                  "example.com",
		"  Example.COM  ":              "example.com",
		"example.com.":                 "example.com",
		"https://example.com":          "example.com",
		"http://example.com/":          "example.com",
		"example.com:8080":             "example.com",
		"https://example.com/pfad?x=1": "example.com",
		"sub.example.com":              "sub.example.com",
		"localhost":                    "localhost",
		"my-site.example.co.uk":        "my-site.example.co.uk",
		"xn--mbel-5qa.de":              "xn--mbel-5qa.de",
		// Internationalised names must be stored the way a browser sends them.
		"möbel.de":      "xn--mbel-5qa.de",
		"MÜNCHEN.de":    "xn--mnchen-3ya.de",
		"grüße.example": "xn--gre-6ka8l.example",
		// A pasted URL is reduced to its host rather than rejected.
		"example.com/../etc": "example.com",
	}

	for in, want := range accepted {
		got, err := NormalizeDomain(in)
		if err != nil {
			t.Errorf("NormalizeDomain(%q) returned error: %v", in, err)
			continue
		}
		if got != want {
			t.Errorf("NormalizeDomain(%q) = %q; want %q", in, got, want)
		}
	}

	rejected := []string{
		"",
		"   ",
		"exa mple.com",
		"example..com",
		"-example.com",
		"example-.com",
		"exa_mple.com",
	}
	for _, in := range rejected {
		if got, err := NormalizeDomain(in); err == nil {
			t.Errorf("NormalizeDomain(%q) = %q, nil; want an error", in, got)
		}
	}
}

// Whatever is stored must be exactly what the resolver compares against, so a
// normalised value has to survive a second pass unchanged.
func TestNormalizeDomainIsIdempotent(t *testing.T) {
	for _, in := range []string{"Example.COM:443", "möbel.de", "example.com."} {
		once, err := NormalizeDomain(in)
		if err != nil {
			t.Fatalf("NormalizeDomain(%q): %v", in, err)
		}
		twice, err := NormalizeDomain(once)
		if err != nil {
			t.Fatalf("NormalizeDomain(%q): %v", once, err)
		}
		if once != twice {
			t.Errorf("not idempotent: %q -> %q -> %q", in, once, twice)
		}
	}
}

func TestDisplayDomain(t *testing.T) {
	cases := map[string]string{
		"xn--mbel-5qa.de": "möbel.de",
		"example.com":     "example.com",
	}
	for in, want := range cases {
		if got := DisplayDomain(in); got != want {
			t.Errorf("DisplayDomain(%q) = %q; want %q", in, got, want)
		}
	}
}
