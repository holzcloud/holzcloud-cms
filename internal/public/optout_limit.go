package public

import (
	"container/list"
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
// A request spends from two networks at once, and has to find room in both
// (WR-01 of the second 33.1 review). The first is the whole IPv4 address, as
// the login throttle keys, or the /64 of an IPv6 address: a subscriber picks
// the low 64 bits freely, so a key on the whole address would be a new budget
// per request. The second is the block around it — the /24, the /48 — with a
// budget of its own. Without it the first was a budget per /64, and a home
// connection is commonly handed a /56, a tunnel broker a /48 for free: one
// subscriber with thirty /64s of their own reached the website's ceiling. The
// block's budget is shared by the neighbours in it, which is the price; a few
// hundred households of one provider sending more than thirty opt-outs to one
// website within an hour is not a thing that happens.
//
// No budget is kept for an address that names no visitor: loopback, a private
// network, or nothing readable. That is what a reverse proxy looks like when
// HOLZCLOUD_TRUSTED_PROXIES does not name it, and there every visitor would
// share one budget — the lock-out this is meant to prevent. Those requests
// meet the website's ceiling only.

// netBudget counts, per network, the requests taken in the last window.
//
// It has a size, maxNets networks, because every network that posts adds one
// and an hour of networks is as many as somebody can bring. Past the size the
// network spent from longest ago is forgotten first: that is the one least
// likely to be the one still posting, and an attacker who keeps posting keeps
// their own block's entry fresh. order holds the networks most recently spent
// from at the front, so the one to forget and the ones the window has passed
// are at the back.
type netBudget struct {
	mu      sync.Mutex
	taken   map[netip.Prefix]*list.Element // of *netSpent
	order   *list.List
	maxNets int
	window  time.Duration
}

// netSpent is one network's requests in the window, oldest first.
type netSpent struct {
	network netip.Prefix
	times   []time.Time
}

// netLimit is a network a request spends from and how many it may spend
// there in a window.
type netLimit struct {
	network netip.Prefix
	max     int
}

func newNetBudget(maxNets int, window time.Duration) *netBudget {
	return &netBudget{
		taken: map[netip.Prefix]*list.Element{}, order: list.New(),
		maxNets: maxNets, window: window,
	}
}

// take spends one request from every network and reports whether each had
// one to spend. When one has not, nothing is spent anywhere and no network
// is added. The check and the spending are one step under the lock, so a
// burst from one network cannot all pass the check before any of it is
// counted.
func (b *netBudget) take(limits []netLimit, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.sweepLocked(now)
	for _, l := range limits {
		if len(b.recentLocked(l.network, now)) >= l.max {
			return false
		}
	}
	for _, l := range limits {
		if el, ok := b.taken[l.network]; ok {
			spent := el.Value.(*netSpent)
			spent.times = append(spent.times, now)
			b.order.MoveToFront(el)
			continue
		}
		b.taken[l.network] = b.order.PushFront(&netSpent{network: l.network, times: []time.Time{now}})
	}
	for b.order.Len() > b.maxNets {
		b.forgetLocked(b.order.Back())
	}
	return true
}

// give returns what take spent at the moment now, for a request that was
// not stored after all. A network left with nothing in the window is
// forgotten, so a refusal leaves no trace behind.
func (b *netBudget) give(limits []netLimit, now time.Time) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, l := range limits {
		el, ok := b.taken[l.network]
		if !ok {
			continue
		}
		spent := el.Value.(*netSpent)
		for i := len(spent.times) - 1; i >= 0; i-- {
			if spent.times[i].Equal(now) {
				spent.times = append(spent.times[:i], spent.times[i+1:]...)
				break
			}
		}
		if len(spent.times) == 0 {
			b.forgetLocked(el)
		}
	}
}

// recentLocked is a network's requests in the window, after dropping the
// older ones; a network with none left is forgotten.
func (b *netBudget) recentLocked(network netip.Prefix, now time.Time) []time.Time {
	el, ok := b.taken[network]
	if !ok {
		return nil
	}
	spent := el.Value.(*netSpent)
	cutoff := now.Add(-b.window)
	i := 0
	for i < len(spent.times) && !spent.times[i].After(cutoff) {
		i++
	}
	spent.times = spent.times[i:]
	if len(spent.times) == 0 {
		b.forgetLocked(el)
		return nil
	}
	return spent.times
}

// sweepLocked forgets the networks at the back of the order whose last
// request the window has passed, so the budget holds the last hour's
// networks and not every one since the start. It stops at the first network
// still in the window: everything in front of it was spent from later.
func (b *netBudget) sweepLocked(now time.Time) {
	cutoff := now.Add(-b.window)
	for el := b.order.Back(); el != nil; el = b.order.Back() {
		spent := el.Value.(*netSpent)
		if len(spent.times) > 0 && spent.times[len(spent.times)-1].After(cutoff) {
			return
		}
		b.forgetLocked(el)
	}
}

func (b *netBudget) forgetLocked(el *list.Element) {
	delete(b.taken, el.Value.(*netSpent).network)
	b.order.Remove(el)
}

// optoutNetworks are the networks a request's budget is spent from, and
// false when it has none (see above).
func (h *Handler) optoutNetworks(r *http.Request) ([]netLimit, bool) {
	raw := r.RemoteAddr
	if h.clientIP != nil {
		raw = h.clientIP.ClientIP(r)
	}
	if host, _, err := net.SplitHostPort(raw); err == nil {
		raw = host
	}
	addr, err := netip.ParseAddr(raw)
	if err != nil {
		return nil, false
	}
	addr = addr.Unmap().WithZone("")
	if addr.IsLoopback() || addr.IsPrivate() || addr.IsUnspecified() || addr.IsLinkLocalUnicast() {
		return nil, false
	}
	own, block := 32, 24
	if addr.Is6() {
		own, block = 64, 48
	}
	return []netLimit{
		{netip.PrefixFrom(addr, own).Masked(), optoutNetworkLimit},
		{netip.PrefixFrom(addr, block).Masked(), optoutBlockLimit},
	}, true
}
