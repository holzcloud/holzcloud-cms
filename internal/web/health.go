package web

import (
	"context"
	"encoding/json"
	"net/http"
	"sync/atomic"
	"time"
)

// ReadinessProbe answers the question /healthz cannot: is this process able to
// serve requests right now.
//
// /healthz stays a constant 200 — that is correct liveness behaviour for
// systemd. A constant string is useless as a readiness signal though: it kept
// reporting ok while the database was corrupt and the disk was full.
type ReadinessProbe struct {
	Version string
	Commit  string

	// Ping verifies both pools are usable.
	Ping func(ctx context.Context) error
	// Stats reports database sizes.
	Stats func(ctx context.Context) (map[string]int64, error)
	// FreeBytes reports free space on the data directory.
	FreeBytes func() (uint64, error)
	// WriteProbe creates and removes a file, catching a read-only remount.
	WriteProbe func() error
	// MinFreeBytes is the floor below which the service reports unready.
	MinFreeBytes uint64

	startedAt time.Time
	// integrity is the start-up quick_check result, cached because running it
	// per request is far too slow on a database of any size.
	integrity atomic.Value
}

// NewReadinessProbe records the start time and the integrity verdict.
func NewReadinessProbe(version, commit, integrity string) *ReadinessProbe {
	p := &ReadinessProbe{Version: version, Commit: commit, startedAt: time.Now()}
	p.integrity.Store(integrity)
	return p
}

// SetIntegrity updates the cached verdict, e.g. after the maintenance job runs.
func (p *ReadinessProbe) SetIntegrity(result string) { p.integrity.Store(result) }

type readyResponse struct {
	Status        string           `json:"status"`
	Version       string           `json:"version"`
	Commit        string           `json:"commit"`
	UptimeSeconds int64            `json:"uptime_s"`
	Integrity     string           `json:"integrity"`
	DiskFreeBytes uint64           `json:"disk_free_bytes"`
	Database      map[string]int64 `json:"database,omitempty"`
	Problems      []string         `json:"problems,omitempty"`
}

// Handler serves GET /readyz.
func (p *ReadinessProbe) Handler() http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()

		resp := readyResponse{
			Status:        "ok",
			Version:       p.Version,
			Commit:        p.Commit,
			UptimeSeconds: int64(time.Since(p.startedAt).Seconds()),
		}
		if v, ok := p.integrity.Load().(string); ok {
			resp.Integrity = v
			if v != "ok" && v != "" {
				resp.Problems = append(resp.Problems, "database integrity: "+v)
			}
		}

		if p.Ping != nil {
			if err := p.Ping(ctx); err != nil {
				resp.Problems = append(resp.Problems, "database unreachable: "+err.Error())
			}
		}
		if p.Stats != nil {
			if stats, err := p.Stats(ctx); err != nil {
				resp.Problems = append(resp.Problems, "database stats: "+err.Error())
			} else {
				resp.Database = stats
			}
		}
		if p.FreeBytes != nil {
			free, err := p.FreeBytes()
			if err != nil {
				resp.Problems = append(resp.Problems, "cannot stat data directory: "+err.Error())
			} else {
				resp.DiskFreeBytes = free
				if p.MinFreeBytes > 0 && free < p.MinFreeBytes {
					resp.Problems = append(resp.Problems,
						"disk space below the configured floor")
				}
			}
		}
		if p.WriteProbe != nil {
			if err := p.WriteProbe(); err != nil {
				resp.Problems = append(resp.Problems, "data directory not writable: "+err.Error())
			}
		}

		status := http.StatusOK
		if len(resp.Problems) > 0 {
			resp.Status = "unready"
			status = http.StatusServiceUnavailable
		}

		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(status)
		_ = json.NewEncoder(w).Encode(resp)
	}
}
