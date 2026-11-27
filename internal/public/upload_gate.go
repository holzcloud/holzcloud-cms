package public

import (
	"net/http"
	"sync"
	"time"
)

// How many submissions with files a public form takes in at once, and for how
// long a refused one is told to wait. Every number is a starting number: the
// least that lets an honest form through on a small server, with the reason
// beside it, and a constant rather than a setting because nobody has yet had a
// reason to turn one (owner decision of 2026-11-27).
const (
	// uploadSlots: the disk a burst can hold is this many times the largest
	// submission, about 300 MB at the default limits, which stays under the
	// 512 MB free-space floor that guards the same disk. Four is also what the
	// plugin runtime's queue can usefully serve.
	uploadSlots = 4
	// uploadNetworkSlots: one person submitting while a second person in the
	// same office does. A third at once from one network is not an office.
	uploadNetworkSlots = 2
	// uploadNetworkPerHour: the contact form's own ceiling of stored messages
	// per website and hour. A network that sends more than that is not
	// sending messages.
	uploadNetworkPerHour = 30
	// uploadRetryAfter: a normal upload is done well within ten seconds, so a
	// visitor who is told to come back after that finds a free slot more often
	// than not.
	uploadRetryAfter = 10 * time.Second
)

// uploadGate bounds the submissions with files that are inside at once, on the
// whole server and from one network.
//
// It counts what is in flight and not what arrived within a time, because that
// is the thing that costs: a slow sender holds spool space on the disk and a
// goroutine for as long as the read timeout allows, and a budget over an hour
// does not stop four of them at once. The hourly budget of a network is a
// different question and is a throttle of its own (attachment.go).
//
// There is no waiting. A stranger's upload is answered at once, 503 when the
// server is full and 429 when the network is, with a time to come back; a queue
// of waiting uploads would be one more thing a stranger can fill.
//
// A submission enters at its first file part (takeAttachments), so a message
// without a file is never counted here.
//
// What it costs: four slow senders keep the attachments of every form on the
// server busy for up to two minutes. Messages without files and the rest of
// the site are untouched.
type uploadGate struct {
	mu         sync.Mutex
	slots      int
	perNetwork int
	inside     int
	// networks is the number inside per network key. A key is deleted when it
	// reaches zero, so the map holds at most as many keys as there are
	// submissions inside.
	networks map[string]int
}

func newUploadGate(slots, perNetwork int) *uploadGate {
	return &uploadGate{slots: slots, perNetwork: perNetwork, networks: map[string]int{}}
}

// enter takes a slot for a submission from network, or says why not: 503 when
// the server is full, 429 when the network is at its bound. An empty network
// skips the network bound, which is what an address that names no visitor gets.
//
// leave gives the slot back; calling it again does nothing.
func (g *uploadGate) enter(network string) (leave func(), status int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	if g.inside >= g.slots {
		return nil, http.StatusServiceUnavailable
	}
	if network != "" && g.networks[network] >= g.perNetwork {
		return nil, http.StatusTooManyRequests
	}
	g.inside++
	if network != "" {
		g.networks[network]++
	}
	var once sync.Once
	return func() {
		once.Do(func() {
			g.mu.Lock()
			defer g.mu.Unlock()
			g.inside--
			if network == "" {
				return
			}
			if g.networks[network]--; g.networks[network] <= 0 {
				delete(g.networks, network)
			}
		})
	}, 0
}
