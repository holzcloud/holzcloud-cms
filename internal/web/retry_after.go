package web

import (
	"net/http"
	"strconv"
	"time"
)

// RetryAfter sets the Retry-After header to d in whole seconds, rounded up and
// at least 1. It is the one way every refusal of the password checks and the
// throttles writes the header, so none of them sends "0" or a fraction.
func RetryAfter(w http.ResponseWriter, d time.Duration) {
	secs := int((d + time.Second - 1) / time.Second)
	if secs < 1 {
		secs = 1
	}
	w.Header().Set("Retry-After", strconv.Itoa(secs))
}
