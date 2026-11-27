package web

import (
	"net/http"
	"strconv"
	"time"
)

// RetryAfter sets the Retry-After header to d in whole seconds, rounded up and
// at least 1. It is the one way a refusal of the password checks and the
// throttles writes the header, so none of them sends "0" or a fraction.
//
// It is written on the answers that carry their own status: 503 for a busy
// gate (login, confirmation, page gate, ErrHandler) and 429 for a full
// throttle at the confirmation and the page gate. The throttled login, second
// step and own-password change answer with a redirect and a flash that names
// the wait instead; a Retry-After on a 303 means nothing to a browser, so they
// do not send one.
func RetryAfter(w http.ResponseWriter, d time.Duration) {
	secs := int((d + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
}
