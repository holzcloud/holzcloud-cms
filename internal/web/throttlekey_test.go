package web

import "testing"

func TestThrottleKey(t *testing.T) {
	cases := []struct{ in, want string }{
		{"203.0.113.7", "203.0.113.7"},
		{"203.0.113.7:4711", "203.0.113.7"},
		{"::ffff:203.0.113.7", "203.0.113.7"},
		{"2001:db8:1:2:aaaa::1", "2001:db8:1:2::/64"},
		{"2001:db8:1:2:bbbb::2", "2001:db8:1:2::/64"},
		{"[2001:db8::1]:443", "2001:db8::/64"},
		{"fe80::1%eth0", "fe80::/64"},
	}
	for _, c := range cases {
		if got := ThrottleKey(c.in); got != c.want {
			t.Errorf("ThrottleKey(%q) = %q, want %q", c.in, got, c.want)
		}
	}

	if ThrottleKey("2001:db8:1:3::1") == ThrottleKey("2001:db8:1:2::1") {
		t.Error("two different /64 networks share a key")
	}

	long := make([]byte, 200)
	for i := range long {
		long[i] = 'x'
	}
	if got := ThrottleKey(string(long)); len(got) > 64 {
		t.Errorf("an unparsable value keeps %d bytes, want at most 64", len(got))
	}
}
