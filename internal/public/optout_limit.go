package public

import (
	"net"
	"net/http"
	"net/netip"
	"sync"
	"time"
)

// The opt-out form's per-network budget (WR-01 of the 33.1 review).
//
// The website's hourly ceiling protects the disk, and it is a count in the
// database. Alone it was also a lever: a single client posting thirty times
// an hour closed the form to every real visitor of the website, and the form
// is one of the ways a US opt-out must be accepted (CCPA regs § 7026(a)(1)).
// So a network has a budget of its own, far below the website's ceiling, and
// spending it closes the form to that network only.
//
// In memory and never in the database, because the form promises to store no
// IP address and the request table has none. A restart forgets the budgets;
// the website's ceiling, which is in the database, does not reset.
//
// The key is the whole IPv4 address, as the login throttle keys, and the /64
// of an IPv6 address: a subscriber picks the low 64 bits freely, so a key on
// the whole address would be a new budget per request.
//
// No budget is kept for an address that names no visitor: loopback, a private
// network, or nothing readable. That is what a reverse proxy looks like when
// HOLZCLOUD_TRUSTED_PROXIES does not name it, and there every visitor would
// share one budget — the lock-out this is meant to prevent. Those requests
// meet the website's ceiling only.

// netBudget counts, per network, how many requests were taken in the last
// window.
type netBudget struct {
	mu     sync.Mutex
	taken  map[netip.Prefix][]time.Time
	max    int
	window time.Duration
	calls  int
}

func newNetBudget(max int, window time.Duration) *netBudget {
	return &netBudget{taken: map[netip.Prefix][]time.Time{}, max: max, window: window}
}

// take spends one request of the network's budget and reports whether there
// was one to spend. The check and the spending are one step under the lock,
// so a burst from one network cannot all pass the check before any of it is
// counted.
func (b *netBudget) take(network netip.Prefix, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.calls++
	if b.calls%256 == 0 {
		b.sweepLocked(now)
	}
	recent := b.pruneLocked(network, now)
	if len(recent) >= b.max {
		return false
	}
	b.taken[network] = append(recent, now)
	return true
}

func (b *netBudget) pruneLocked(network netip.Prefix, now time.Time) []time.Time {
	cutoff := now.Add(-b.window)
	times := b.taken[network]
	i := 0
	for i < len(times) && !times[i].After(cutoff) {
		i++
	}
	times = times[i:]
	if len(times) == 0 {
		delete(b.taken, network)
		return nil
	}
	b.taken[network] = times
	return times
}

// sweepLocked drops every network whose requests are all older than the
// window, so the map holds the last hour's networks and not every one since
// the start.
func (b *netBudget) sweepLocked(now time.Time) {
	for network := range b.taken {
		b.pruneLocked(network, now)
	}
}

// optoutNetwork is the network a request's budget is kept under, and false
// when it has none (see above).
func (h *Handler) optoutNetwork(r *http.Request) (netip.Prefix, bool) {
	raw := r.RemoteAddr
	if h.clientIP != nil {
		raw = h.clientIP.ClientIP(r)
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return netip.Prefix{}, false
	}
	addr = addr.Unmap().WithZone("")
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() || addr.IsLinkLocalUnicast() {
		return netip.Prefix{}, false
	}
	bits := 32
	if addr.Is6() {
		bits = 64
	}
	return netip.PrefixFrom(addr, bits).Masked(), true
}
