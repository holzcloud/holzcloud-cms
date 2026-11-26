package web

import "testing"

func TestRedirectTarget(t *testing.T) {
	accepted := []string{
		"/kontakt",
		"/fr/neu#hier",
		"https://example.com/weg",
		"HTTP://example.com/x",
		"/aktion-50%",
		"/aktion-50%?x=1",
	}
	for _, to := range accepted {
		got, ok := RedirectTarget(to)
		if !ok || got != to {
			t.Errorf("RedirectTarget(%q) = %q, %v; want it accepted unchanged", to, got, ok)
		}
	}

	refused := []string{
		"//evil.example/x",
		`/\evil.example`,
		"/\t/evil.example",
		"\t/kontakt",
		`https://evil.example\@ok.example/`,
		"https:///nohost",
		"https://user@example.com/x",
		"javascript:alert(1)",
		"ftp://example.com/",
		"",
	}
	for _, to := range refused {
		if got, ok := RedirectTarget(to); ok || got != "" {
			t.Errorf("RedirectTarget(%q) = %q, %v; want it refused", to, got, ok)
		}
	}
}
