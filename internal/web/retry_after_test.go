package web

import (
	"net/http/httptest"
	"testing"
	"time"
)

func TestRetryAfterIsWholeSecondsRoundedUpAndAtLeastOne(t *testing.T) {
	for _, c := range []struct {
		d    time.Duration
		want string
	}{
		{0, "1"}, {-time.Second, "1"}, {time.Millisecond, "1"}, {2 * time.Second, "2"},
		{2*time.Second + time.Millisecond, "3"}, {15 * time.Minute, "900"},
	} {
		rec := httptest.NewRecorder()
		RetryAfter(rec, c.d)
		if got := rec.Header().Get("Retry-After"); got != c.want {
			t.Errorf("RetryAfter(%v) = %q, want %q", c.d, got, c.want)
		}
	}
}
