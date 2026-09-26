package web

import "testing"

// The notice follows the version number: before 1.0, and for anything that is
// not a version at all, it is shown; from 1.0 on it is not.
func TestIsAlpha(t *testing.T) {
	for v, want := range map[string]bool{
		"dev": true, "": true, "a1b2c3d": true,
		"v0.0.1": true, "0.3": true, "v0.9.2-4-gabc1234": true,
		"v1.0": false, "v1.0.0": false, "2.9.1": false, "v10.2": false,
	} {
		if got := IsAlpha(v); got != want {
			t.Errorf("IsAlpha(%q) = %v, want %v", v, got, want)
		}
	}
}
