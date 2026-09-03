package web

import (
	"bufio"
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"log/slog"
	"net"
	"net/http"
	"runtime/debug"
	"time"
)

type contextKey struct{ name string }

var requestIDKey = &contextKey{"request-id"}

// RequestIDFromContext returns the id assigned to the current request.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}

// RequestID assigns every request an identifier and echoes it back.
//
// An incoming X-Request-ID is only adopted from a trusted proxy; otherwise a
// client could poison the logs by choosing its own, or by reusing someone
// else's to make two unrelated requests look like one.
func RequestID(resolver *ClientIPResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			id := ""
			if resolver != nil && resolver.IsTrustedPeer(r) {
				id = sanitizeRequestID(r.Header.Get("X-Request-ID"))
			}
			if id == "" {
				id = newRequestID()
			}
			w.Header().Set("X-Request-ID", id)
			next.ServeHTTP(w, r.WithContext(context.WithValue(r.Context(), requestIDKey, id)))
		})
	}
}

func newRequestID() string {
	b := make([]byte, 8)
	if _, err := rand.Read(b); err != nil {
		return "unknown"
	}
	return hex.EncodeToString(b)
}

// sanitizeRequestID keeps a forwarded id only if it is short and printable, so
// a log line cannot be forged with newlines or control characters.
func sanitizeRequestID(id string) string {
	if len(id) == 0 || len(id) > 64 {
		return ""
	}
	for i := 0; i < len(id); i++ {
		c := id[i]
		ok := (c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') ||
			(c >= '0' && c <= '9') || c == '-' || c == '_'
		if !ok {
			return ""
		}
	}
	return id
}

// responseRecorder captures the status and byte count for the access log.
//
// It forwards Flush and Hijack: without them a wrapped ResponseWriter silently
// breaks streaming and connection upgrades, and the breakage only shows up at
// runtime.
type responseRecorder struct {
	http.ResponseWriter
	status int
	bytes  int64
}

func (r *responseRecorder) WriteHeader(status int) {
	if r.status == 0 {
		r.status = status
	}
	r.ResponseWriter.WriteHeader(status)
}

func (r *responseRecorder) Write(b []byte) (int, error) {
	if r.status == 0 {
		r.status = http.StatusOK
	}
	n, err := r.ResponseWriter.Write(b)
	r.bytes += int64(n)
	return n, err
}

func (r *responseRecorder) Flush() {
	if f, ok := r.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}

func (r *responseRecorder) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := r.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, errors.New("ResponseWriter does not support hijacking")
}

func (r *responseRecorder) Unwrap() http.ResponseWriter { return r.ResponseWriter }

// AccessLog writes one structured record per request.
//
// Successful requests log at Debug so a busy site does not fill the journal;
// client and server errors log at Warn and Error, which is what an operator
// actually needs to find.
func AccessLog(resolver *ClientIPResolver) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &responseRecorder{ResponseWriter: w}

			next.ServeHTTP(rec, r)

			if rec.status == 0 {
				rec.status = http.StatusOK
			}
			ip := r.RemoteAddr
			if resolver != nil {
				ip = resolver.ClientIP(r)
			}

			level := slog.LevelDebug
			switch {
			case rec.status >= 500:
				level = slog.LevelError
			case rec.status >= 400:
				level = slog.LevelWarn
			}

			slog.LogAttrs(r.Context(), level, "request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.Int("status", rec.status),
				slog.Int64("bytes", rec.bytes),
				slog.Int64("duration_ms", time.Since(start).Milliseconds()),
				slog.String("host", r.Host),
				slog.String("ip", ip),
				slog.String("request_id", RequestIDFromContext(r.Context())),
			)
		})
	}
}

// Recoverer turns a panic into a logged 500 instead of a dropped connection.
//
// It sits inside AccessLog so a panicking request still produces an access
// record, and it reports the request id so the two lines can be tied together.
func Recoverer(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			rec := recover()
			if rec == nil {
				return
			}
			// A client that goes away mid-write is not a bug worth a stack trace.
			if err, ok := rec.(error); ok && errors.Is(err, http.ErrAbortHandler) {
				panic(rec)
			}
			slog.Error("panic recovered",
				"err", rec,
				"method", r.Method,
				"path", r.URL.Path,
				"request_id", RequestIDFromContext(r.Context()),
				"stack", string(debug.Stack()),
			)
			http.Error(w, "Internal Server Error", http.StatusInternalServerError)
		}()
		next.ServeHTTP(w, r)
	})
}
