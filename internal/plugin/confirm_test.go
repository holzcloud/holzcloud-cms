package plugin

import (
	"context"
	"encoding/json"
	"testing"
)

// A plugin with "notify" alone cannot reach a third party.
//
// This is the bound PermConfirm's comment promises, and it is the only thing
// standing between a contact form and a mail relay: asking for the copy is not
// being allowed it. The check is in the runtime, before the host function is
// called, so a host that forgot the check is still safe.
func TestAskingForACopyNeedsTheConfirmPermission(t *testing.T) {
	for _, c := range []struct {
		name  string
		perms []string
		asked bool
		want  bool
	}{
		{"notify alone, asking", []string{PermNotify}, true, false},
		{"notify and confirm, asking", []string{PermNotify, PermConfirm}, true, true},
		{"notify and confirm, not asking", []string{PermNotify, PermConfirm}, false, false},
		{"confirm without notify, asking", []string{PermConfirm}, true, true},
	} {
		t.Run(c.name, func(t *testing.T) {
			var got NotifyArg
			r := &Runtime{
				notify: func(_ context.Context, _ int64, a NotifyArg) (bool, bool, string, error) {
					got = a
					return true, a.Confirm, "", nil
				},
			}
			cc := &callCtx{
				pluginID:  "demo",
				manifest:  &Manifest{ID: "demo", Permissions: c.perms},
				websiteID: 1,
			}
			arg, _ := json.Marshal(NotifyArg{
				Subject: "x", Body: "y", ReplyTo: "anna@example.test", Confirm: c.asked,
			})
			raw, err := r.runOp(context.Background(), cc, OpNotify, arg)
			if err != nil {
				t.Fatalf("perform: %v", err)
			}
			if got.Confirm != c.want {
				t.Errorf("the host was asked for a copy = %v, want %v", got.Confirm, c.want)
			}
			var res NotifyResult
			if err := json.Unmarshal(raw, &res); err != nil {
				t.Fatal(err)
			}
			if res.Confirmed != c.want {
				t.Errorf("Confirmed = %v, want %v", res.Confirmed, c.want)
			}
		})
	}
}
