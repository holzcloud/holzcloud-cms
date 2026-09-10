package main

import (
	"strings"
	"testing"
)

// TestTwoFactorDisabledNoticeFollowsTheRule: what `holzcloud user 2fa disable`
// promises must be what the second-factor rule does, for both roles and with
// single sign-on on and off.
func TestTwoFactorDisabledNoticeFollowsTheRule(t *testing.T) {
	for _, tc := range []struct {
		name         string
		role         string
		sso          bool
		wantSetup    bool
		wantProvider bool
	}{
		{"an administrator, single sign-on off", "admin", false, true, false},
		{"an administrator, single sign-on on", "admin", true, true, true},
		{"an editor, single sign-on on", "editor", true, false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			text := strings.Join(twoFactorDisabledNotice("ada@example.com", tc.role, tc.sso), "\n")
			if got := strings.Contains(text, "set up a new authenticator"); got != tc.wantSetup {
				t.Errorf("promises a new authenticator = %v; want %v\n%s", got, tc.wantSetup, text)
			}
			if got := strings.Contains(text, "identity provider"); got != tc.wantProvider {
				t.Errorf("mentions the identity provider = %v; want %v\n%s", got, tc.wantProvider, text)
			}
		})
	}
}
