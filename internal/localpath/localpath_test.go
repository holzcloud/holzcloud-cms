package localpath

import (
	"net/url"
	"strings"
	"testing"
)

func TestIs(t *testing.T) {
	cases := []struct {
		name string
		in   string
		want bool
	}{
		{"root", "/", true},
		{"path", "/kontakt", true},
		{"query and fragment", "/shop/tisch?x=1#y", true},
		{"language prefix", "/fr/geheim", true},
		{"inner double slash", "/a//b", true},
		{"encoded space", "/ok%20space", true},
		{"query flag", "/kontakt?gesendet=1", true},

		{"empty", "", false},
		{"no leading slash", "kontakt", false},
		{"protocol relative", "//evil.example", false},
		{"slash backslash", `/\evil.example`, false},
		{"two backslashes", `\\evil.example`, false},
		{"inner backslash", `/x\y`, false},
		{"tab", "/\tevil.example", false},
		{"crlf", "/x\r\nSet-Cookie: a=b", false},
		{"del", "/x\x7f", false},
		{"encoded slash", "/%2F%2Fevil.example", false},
		{"encoded backslash", "/%5cevil.example", false},
		{"absolute url", "https://evil.example/", false},
		{"javascript", "javascript:alert(1)", false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if got := Is(c.in); got != c.want {
				t.Errorf("Is(%q) = %v, want %v", c.in, got, c.want)
			}
		})
	}
}

// FuzzIs holds the property the predicate exists for: whatever it accepts, a
// browser resolves against this site and stays on it.
func FuzzIs(f *testing.F) {
	for _, s := range []string{
		"/", "/kontakt", "/shop/tisch?x=1#y", "/fr/geheim", "/a//b", "/ok%20space",
		"/kontakt?gesendet=1", "", "kontakt", "//evil.example", `/\evil.example`,
		`\\evil.example`, `/x\y`, "/\tevil.example", "/x\r\nSet-Cookie: a=b", "/x\x7f",
		"/%2F%2Fevil.example", "/%5cevil.example", "https://evil.example/", "javascript:alert(1)",
	} {
		f.Add(s)
	}
	base, err := url.Parse("https://example.ch/a/b")
	if err != nil {
		f.Fatal(err)
	}
	f.Fuzz(func(t *testing.T, p string) {
		if !Is(p) {
			return
		}
		u, err := url.Parse(p)
		if err != nil {
			t.Fatalf("Is(%q) is true but url.Parse fails: %v", p, err)
		}
		if u.Host != "" {
			t.Fatalf("Is(%q) is true but the host is %q", p, u.Host)
		}
		// A browser removes tab, line feed and carriage return and reads
		// every backslash as a slash before it resolves.
		seen := strings.NewReplacer("\t", "", "\n", "", "\r", "", `\`, "/").Replace(p)
		ref, err := url.Parse(seen)
		if err != nil {
			t.Fatalf("Is(%q) is true but the browser's reading %q does not parse: %v", p, seen, err)
		}
		if got := base.ResolveReference(ref); got.Host != "example.ch" {
			t.Fatalf("Is(%q) is true but a browser lands on host %q", p, got.Host)
		}
	})
}
