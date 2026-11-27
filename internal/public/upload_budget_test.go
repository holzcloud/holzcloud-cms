package public

import (
	"fmt"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
)

// The hourly budget of one network cannot be turned into a lock-out of every
// other network (CR-01 of the phase 38 review).
//
// The budget kept at most auth.DefaultMaxKeys networks and refused a new one
// while it held that many, a rule made for the login's account axis. With one
// IPv6 /48, 65,536 networks of its own, a stranger filled it with ten thousand
// tiny submissions in under a second, and for the next hour every network not
// yet in it was refused 429 on every website. The upload budget has no account
// axis, so it makes room by forgetting the network spent from longest ago; the
// gate still bounds what is inside at once.
func TestTheHourlyUploadBudgetCannotBeFilledByNewNetworks(t *testing.T) {
	h, ws := behindAProxy(t)
	for i := 0; i < auth.DefaultMaxKeys; i++ {
		a, wait := h.uploadNets.Reserve(fmt.Sprintf("2001:db8:%x:%x::/64", i>>16, i&0xffff), "")
		if a == nil {
			t.Fatalf("filling: network %d refused with %v", i, wait)
		}
		a.Fail()
	}
	keeper, refused := submitFiles(t, h, ws, via("2001:db8:ffff:1::1"))
	if refused != nil {
		t.Fatalf("a fresh network after the fill: refused=%+v, want taken", refused)
	}
	keeper.Close()
}
