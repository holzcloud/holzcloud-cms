package public

import (
	"bytes"
	"fmt"
	"math"
	"net/http/httptest"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
	"github.com/holzcloud/holzcloud-cms/internal/media"
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

// Limits as large as the configuration accepts do not wrap the bound or the
// floor around (IN-05 of the phase 38 review): the sums saturate, so the bound
// stays positive and the floor stays above any free space, which refuses files
// rather than every submission.
func TestExtremeLimitsDoNotOverflowTheBounds(t *testing.T) {
	h, ws := boundsCase(t, math.MaxInt64, math.MaxInt64)
	if b := attachmentBound(media.Limits{Image: math.MaxInt64, Video: math.MaxInt64}); b != math.MaxInt64 {
		t.Errorf("attachmentBound = %d, want it saturated at MaxInt64", b)
	}
	h.SetMinFreeBytes(512 << 20)
	if f := h.AttachmentFloor(); f != math.MaxUint64 {
		t.Errorf("AttachmentFloor = %d, want it saturated at MaxUint64", f)
	}
	// A message still gets through.
	body, ctype := rawMultipart(t, false, fld("name", "Anna"))
	req := httptest.NewRequest("POST", "/formular", bytes.NewReader(body))
	req.Header.Set("Content-Type", ctype)
	fields, keeper, took, refused := h.takeAttachments(httptest.NewRecorder(), req, ws.ID, "kontaktformular", true)
	if keeper != nil {
		keeper.Close()
	}
	if !took || refused != nil || fields != "name=Anna" {
		t.Errorf("took=%v refused=%+v fields=%q", took, refused, fields)
	}
}

// Behind a trusted proxy that passes X-Forwarded-For on without appending to
// it, a sender can put anything there. An address of their choice is a
// network of their choice, as for any spoofed header; but a value that is no
// address at all used to name no network and met only the server's bound
// (I-2 of the phase 38 security audit). All such values now share one network
// and its bounds.
func TestAnUnreadableForwardedAddressSharesOneNetwork(t *testing.T) {
	h, ws := behindAProxy(t)
	for i, xff := range []string{"garbage", "nonsense"} {
		if _, refused := submitFiles(t, h, ws, via(xff)); refused != nil {
			t.Fatalf("unreadable sender %d: refused=%+v", i+1, refused)
		}
	}
	if _, refused := submitFiles(t, h, ws, via("still-not-an-address")); refused == nil || refused.Status != 429 {
		t.Fatalf("a third unreadable sender at once: refused=%+v, want 429", refused)
	}
	// A peer that is no proxy and names no visitor is unchanged: it meets the
	// server bound alone (TestAnAddressThatNamesNoVisitorMeetsOnlyTheServerBound).
}
