package web

import (
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func TestRequestIDIsAssignedAndEchoed(t *testing.T) {
	var seen string
	h := RequestID(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if seen == "" {
		t.Fatal("no request id in the context")
	}
	if got := rec.Header().Get("X-Request-ID"); got != seen {
		t.Errorf("header %q does not match context %q", got, seen)
	}
}

// A client must not be able to choose its own id: it could poison the logs or
// make two unrelated requests look like one.
func TestRequestIDIgnoresUntrustedHeader(t *testing.T) {
	h := RequestID(NewClientIPResolver(nil))(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {}))

	req := httptest.NewRequest("GET", "/", nil)
	req.RemoteAddr = "198.51.100.4:1234"
	req.Header.Set("X-Request-ID", "gefaelscht")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if got := rec.Header().Get("X-Request-ID"); got == "gefaelscht" {
		t.Error("an untrusted client's request id was adopted")
	}
}

func TestRequestIDRejectsUnsafeForwardedValues(t *testing.T) {
	for _, bad := range []string{
		"has space", "with\nnewline", strings.Repeat("x", 65), "semi;colon",
	} {
		if got := sanitizeRequestID(bad); got != "" {
			t.Errorf("sanitizeRequestID(%q) = %q; want rejection", bad, got)
		}
	}
	if got := sanitizeRequestID("abc-123_DEF"); got != "abc-123_DEF" {
		t.Errorf("a safe id was rejected: %q", got)
	}
}

// A panic must become a logged 500, not a dropped connection.
func TestRecovererTurnsPanicInto500(t *testing.T) {
	h := Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("etwas ist kaputt")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500", rec.Code)
	}
	if strings.Contains(rec.Body.String(), "etwas ist kaputt") {
		t.Error("the panic value leaked into the response body")
	}
}

// Wrapping the ResponseWriter must not break streaming — losing Flush is the
// kind of failure that only shows up at runtime.
func TestAccessLogPreservesFlusher(t *testing.T) {
	var flushable bool
	h := AccessLog(nil)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, flushable = w.(http.Flusher)
		w.WriteHeader(http.StatusTeapot)
		w.Write([]byte("hallo"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/", nil))

	if !flushable {
		t.Error("the wrapped ResponseWriter is not an http.Flusher")
	}
	if rec.Code != http.StatusTeapot {
		t.Errorf("status not passed through: %d", rec.Code)
	}
	if rec.Body.String() != "hallo" {
		t.Errorf("body not passed through: %q", rec.Body.String())
	}
}

// A panicking request must still produce an access log line, which is why
// Recoverer sits inside AccessLog.
func TestAccessLogRecordsAPanickingRequest(t *testing.T) {
	h := AccessLog(nil)(Recoverer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		panic("boom")
	})))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest("GET", "/kaputt", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Errorf("status = %d; want 500", rec.Code)
	}
}
