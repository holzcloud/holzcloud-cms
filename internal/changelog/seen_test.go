package changelog

import "testing"

func TestDecide(t *testing.T) {
	list := []string{"0.3", "0.2", "0.1", "0.0.9"}
	cases := []struct {
		name          string
		seen, running string
		want          Outcome
		wantVersion   string
	}{
		{"dev build", "0.1", "dev", Nothing, ""},
		{"git describe suffix", "0.1", "v0.3-3-gabc1234", Nothing, ""},
		{"bare hash", "0.1", "abc1234", Nothing, ""},
		{"new account records", "", "0.3", Record, "0.3"},
		{"already seen", "0.3", "0.3", Nothing, ""},
		{"already seen, v prefix", "0.3", "v0.3", Nothing, ""},
		{"one behind", "0.2", "0.3", Show, "0.3"},
		{"several behind", "0.0.9", "v0.3", Show, "0.3"},
		{"downgrade keeps the highest", "0.3", "0.2", Nothing, ""},
		{"unknown stored value counts as older", "weird", "0.3", Show, "0.3"},
		{"non-release build and empty seen", "", "dev", Nothing, ""},
	}
	for _, c := range cases {
		got, v := decide(list, c.seen, c.running)
		if got != c.want || v != c.wantVersion {
			t.Errorf("%s: decide(%q, %q) = %v, %q; want %v, %q",
				c.name, c.seen, c.running, got, v, c.want, c.wantVersion)
		}
	}
}

func TestDecideThroughTheRealList(t *testing.T) {
	latest, ok := Latest()
	if !ok {
		t.Skip("no releases")
	}
	if got, v := Decide("", latest.Version); got != Record || v != latest.Version {
		t.Errorf("Decide(\"\", latest) = %v, %q", got, v)
	}
	if got, _ := Decide(latest.Version, "v"+latest.Version); got != Nothing {
		t.Errorf("seen == running = %v, want Nothing", got)
	}
	if got, _ := Decide("dev", "dev"); got != Nothing {
		t.Errorf("dev = %v, want Nothing", got)
	}
}
