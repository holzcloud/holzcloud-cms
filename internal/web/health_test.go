package web

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
)

func probeResponse(t *testing.T, p *ReadinessProbe) (int, readyResponse) {
	t.Helper()
	rec := httptest.NewRecorder()
	p.Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))

	var body readyResponse
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("readyz body is not JSON: %v (%s)", err, rec.Body.String())
	}
	return rec.Code, body
}

func healthyProbe() *ReadinessProbe {
	p := NewReadinessProbe("v1.2.3", "abc1234", "ok")
	p.Ping = func(context.Context) error { return nil }
	p.FreeBytes = func() (uint64, error) { return 10 << 30, nil }
	p.WriteProbe = func() error { return nil }
	p.MinFreeBytes = 512 << 20
	return p
}

func TestReadyzReportsHealthy(t *testing.T) {
	code, body := probeResponse(t, healthyProbe())

	if code != http.StatusOK {
		t.Errorf("status = %d; want 200", code)
	}
	if body.Status != "ok" || body.Version != "v1.2.3" || body.Commit != "abc1234" {
		t.Errorf("unexpected body: %+v", body)
	}
	if len(body.Problems) != 0 {
		t.Errorf("healthy probe reported problems: %v", body.Problems)
	}
}

// The whole point of /readyz: unlike /healthz it must go red when the process
// cannot actually serve.
func TestReadyzReports503OnEachFailure(t *testing.T) {
	cases := map[string]func(*ReadinessProbe){
		"database unreachable": func(p *ReadinessProbe) {
			p.Ping = func(context.Context) error { return errors.New("connection refused") }
		},
		"corrupt database": func(p *ReadinessProbe) {
			p.SetIntegrity("*** in database main *** Page 42 is never used")
		},
		"disk below the floor": func(p *ReadinessProbe) {
			p.FreeBytes = func() (uint64, error) { return 1 << 20, nil }
		},
		"read-only filesystem": func(p *ReadinessProbe) {
			p.WriteProbe = func() error { return errors.New("read-only file system") }
		},
	}

	for label, break_ := range cases {
		t.Run(label, func(t *testing.T) {
			p := healthyProbe()
			break_(p)

			code, body := probeResponse(t, p)
			if code != http.StatusServiceUnavailable {
				t.Errorf("status = %d; want 503", code)
			}
			if body.Status != "unready" {
				t.Errorf("status field = %q; want unready", body.Status)
			}
			if len(body.Problems) == 0 {
				t.Error("no problem was named, so an operator learns nothing")
			}
		})
	}
}

// The integrity verdict is cached at startup because running it per request is
// far too slow, but the maintenance job must be able to refresh it.
func TestReadyzIntegrityCanBeRefreshed(t *testing.T) {
	p := healthyProbe()
	p.SetIntegrity("*** in database main ***")

	if code, _ := probeResponse(t, p); code != http.StatusServiceUnavailable {
		t.Fatalf("corrupt integrity did not fail the probe: %d", code)
	}

	p.SetIntegrity("ok")
	if code, _ := probeResponse(t, p); code != http.StatusOK {
		t.Errorf("probe stayed unready after recovery: %d", code)
	}
}

func TestReadyzIsNeverCached(t *testing.T) {
	rec := httptest.NewRecorder()
	healthyProbe().Handler().ServeHTTP(rec, httptest.NewRequest("GET", "/readyz", nil))

	if got := rec.Header().Get("Cache-Control"); got != "no-store" {
		t.Errorf("Cache-Control = %q; want no-store", got)
	}
}

func TestFreeBytesAndWriteProbe(t *testing.T) {
	dir := t.TempDir()

	free, err := FreeBytes(dir)
	if err != nil {
		t.Fatalf("FreeBytes: %v", err)
	}
	if free == 0 {
		t.Error("FreeBytes reported zero for a writable temp directory")
	}
	if err := WriteProbe(dir); err != nil {
		t.Errorf("WriteProbe on a writable directory: %v", err)
	}
	if err := WriteProbe("/nonexistent-holzcloud-directory"); err == nil {
		t.Error("WriteProbe succeeded on a directory that does not exist")
	}
}
