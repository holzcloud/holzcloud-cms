package public

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// held is how many submissions are inside the gate and how many network keys
// it remembers.
func held(g *uploadGate) (inside, keys int) {
	g.mu.Lock()
	defer g.mu.Unlock()
	return g.inside, len(g.networks)
}

// fillTheServer takes every server slot with a network of its own and returns
// the functions that give them back.
func fillTheServer(t *testing.T, g *uploadGate) []func() {
	t.Helper()
	var leaves []func()
	for i := 0; i < uploadSlots; i++ {
		leave, status := g.enter("filler-" + string(rune('a'+i)))
		if leave == nil {
			t.Fatalf("slot %d of %d was refused with %d", i+1, uploadSlots, status)
		}
		leaves = append(leaves, leave)
	}
	return leaves
}

// A server that is taking four submissions with files refuses the fifth at
// once: a 503 that says when to come back, at the header of its first file and
// before a byte of that file is read, and nothing is spooled for it. The fields
// before the file are read: whether a submission carries a file is not known
// until its first file part arrives (WR-01 of the phase 38 review).
func TestAFullServerRefusesAnUploadBeforeReadingItsFile(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	leaves := fillTheServer(t, h.uploads)
	body, ctype := rawMultipart(t, false, fld("name", "Anna"), filePart("anhang", "bild.png", bigPNG(64<<10)))

	counted := &countingBody{r: bytes.NewReader(body)}
	req := httptest.NewRequest("POST", "/formular", nil)
	req.Body = counted
	req.ContentLength = int64(len(body))
	req.Header.Set("Content-Type", ctype)
	_, keeper, took, refused := h.takeAttachments(httptest.NewRecorder(), req, ws.ID, "kontaktformular", true)
	if took || keeper != nil || refused == nil || refused.Status != http.StatusServiceUnavailable {
		if keeper != nil {
			keeper.Close()
		}
		t.Fatalf("with every slot held: took=%v refused=%+v, want 503", took, refused)
	}
	if refused.RetryAfter < time.Second {
		t.Errorf("Retry-After %v, want at least one second", refused.RetryAfter)
	}
	// The fields and the file's header, and what the multipart reader buffers
	// ahead (4 KB), but not the 64 KB of the file.
	if counted.n > 8<<10 {
		t.Errorf("%d bytes of a %d-byte body were read before the refusal: the file was read", counted.n, len(body))
	}
	if names := spoolEntries(t, h.dataDir); len(names) != 0 {
		t.Errorf("the spool holds %v for a refused submission", names)
	}

	leaves[0]()
	if _, keeper, took, refused := takeBody(t, h, ws, body, ctype); !took || refused != nil || len(keeper.List()) != 1 {
		t.Errorf("after a slot was given back: took=%v refused=%+v", took, refused)
	}
}

// The answer through the middleware is the standard status text with
// Retry-After, as for a plugin that is busy.
func TestAFullServerAnswersWithRetryAfterThroughTheMiddleware(t *testing.T) {
	h, ws, _ := echoAttachRoute(t, plugin.Limits{CallTimeout: 500 * time.Millisecond})
	fillTheServer(t, h.uploads)
	body, ctype := rawMultipart(t, false, fld("name", "Anna"), filePart("anhang", "bild.png", bigPNG(64<<10)))
	rec := postEcho(t, h, ws, body, ctype)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("status %d, want 503", rec.Code)
	}
	if n := retryAfter(t, rec); n < 1 {
		t.Errorf("Retry-After %d, want at least 1", n)
	}
	if got := rec.Body.String(); got != http.StatusText(http.StatusServiceUnavailable)+"\n" {
		t.Errorf("the visitor was told %q, want the plain status text", got)
	}
}

// Whatever way a submission leaves, its slot is back.
func TestTheUploadSlotIsGivenBackOnEveryExit(t *testing.T) {
	inside := func(t *testing.T, h *Handler, label string) {
		t.Helper()
		if n, keys := held(h.uploads); n != 0 || keys != 0 {
			t.Errorf("%s: %d submissions inside and %d network keys, want none", label, n, keys)
		}
	}
	picture, ctype := rawMultipart(t, false, filePart("anhang", "bild.png", bigPNG(64<<10)), fld("name", "Anna"))

	t.Run("taken and closed, twice", func(t *testing.T) {
		h, ws := boundsCase(t, 5<<20, 50<<20)
		_, keeper, took, refused := takeBody(t, h, ws, picture, ctype)
		if !took || refused != nil {
			t.Fatalf("took=%v refused=%+v", took, refused)
		}
		if n, _ := held(h.uploads); n != 1 {
			t.Fatalf("%d inside while the keeper is open, want 1", n)
		}
		// Another submission inside shows that the second Close gives back
		// nothing more.
		other, status := h.uploads.enter("other")
		if other == nil {
			t.Fatalf("a second submission was refused with %d", status)
		}
		keeper.Close()
		keeper.Close()
		if n, _ := held(h.uploads); n != 1 {
			t.Errorf("after Close twice %d inside, want the other one only", n)
		}
		other()
		inside(t, h, "taken and closed")
	})

	t.Run("cut body: 400", func(t *testing.T) {
		h, ws := boundsCase(t, 5<<20, 50<<20)
		cut, cutType := rawMultipart(t, true, filePart("anhang", "bild.png", bigPNG(64<<10)), fld("name", "Anna"))
		if _, _, _, refused := takeBody(t, h, ws, cut, cutType); refused == nil || refused.Status != http.StatusBadRequest {
			t.Fatalf("refused=%+v, want 400", refused)
		}
		inside(t, h, "400")
	})

	t.Run("too many parts: 413", func(t *testing.T) {
		h, ws := boundsCase(t, 5<<20, 50<<20)
		many, manyType := rawMultipart(t, false, fieldsOf(maxFormParts+1, 2)...)
		if _, _, _, refused := takeBody(t, h, ws, many, manyType); refused == nil || refused.Status != http.StatusRequestEntityTooLarge {
			t.Fatalf("refused=%+v, want 413", refused)
		}
		inside(t, h, "413")
	})

	t.Run("the plugin declines", func(t *testing.T) {
		h, ws, _ := echoAttachRoute(t, plugin.Limits{CallTimeout: 500 * time.Millisecond, QueueWait: 20 * time.Millisecond})
		if rec := postEcho(t, h, ws, picture, ctype); rec.Code != http.StatusNotFound {
			t.Fatalf("status %d, want 404", rec.Code)
		}
		inside(t, h, "declined")
	})

	t.Run("the plugin is busy", func(t *testing.T) {
		h, ws, manager := echoAttachRoute(t, plugin.Limits{CallTimeout: 500 * time.Millisecond, QueueWait: 20 * time.Millisecond})
		endless(manager, ws.ID)
		deadline := time.Now().Add(3 * time.Second)
		for {
			rec := postEcho(t, h, ws, picture, ctype)
			inside(t, h, "busy loop")
			if rec.Code == http.StatusServiceUnavailable {
				break
			}
			if time.Now().After(deadline) {
				t.Fatalf("the route never answered 503 (last: %d)", rec.Code)
			}
			time.Sleep(5 * time.Millisecond)
		}
	})
}

// A panic between entering and handing the keeper on gives the slot back, with
// the spool.
func TestAPanicInsideTheTakeGivesTheSlotBack(t *testing.T) {
	h, ws := boundsCase(t, 5<<20, 50<<20)
	body, ctype := rawMultipart(t, false,
		filePart("anhang", "a.png", bigPNG(64<<10)),
		filePart("anhang", "b.png", bigPNG(64<<10)))

	var insideAtPanic int
	attachPartHook = func() {
		insideAtPanic, _ = held(h.uploads)
		panic("a part went wrong")
	}
	// A t.Fatal inside the hook would leave it set for every later test.
	t.Cleanup(func() { attachPartHook = nil })
	func() {
		defer func() {
			if recover() == nil {
				t.Error("the hook did not panic: it was never called")
			}
		}()
		takeBody(t, h, ws, body, ctype)
	}()
	attachPartHook = nil

	if insideAtPanic != 1 {
		t.Errorf("%d submissions inside at the panic, want this one: the test shows nothing", insideAtPanic)
	}
	if n, keys := held(h.uploads); n != 0 || keys != 0 {
		t.Errorf("after the panic %d inside and %d keys, want none", n, keys)
	}
	if names := spoolEntries(t, h.dataDir); len(names) != 0 {
		t.Errorf("the spool holds %v after the panic", names)
	}
	if _, keeper, took, refused := takeBody(t, h, ws, body, ctype); !took || refused != nil || len(keeper.List()) != 2 {
		t.Errorf("the next submission: took=%v refused=%+v", took, refused)
	}
}

// Run under -race: however many come at once, no more than the slots are
// inside, no more than the network's bound from one network, and the gate is
// empty afterwards.
func TestTheGateNeverAdmitsMoreThanItsSlots(t *testing.T) {
	g := newUploadGate(uploadSlots, uploadNetworkSlots)
	var (
		mu       sync.Mutex
		now      int
		perKey   = map[string]int{}
		mostAll  int
		mostKey  int
		admitted atomic.Int64
		refused  atomic.Int64
	)
	start := make(chan struct{})
	var wg sync.WaitGroup
	for i := 0; i < 64; i++ {
		key := "network-" + string(rune('a'+i%8))
		wg.Add(1)
		go func() {
			defer wg.Done()
			<-start
			for round := 0; round < 20; round++ {
				leave, _ := g.enter(key)
				if leave == nil {
					refused.Add(1)
					time.Sleep(200 * time.Microsecond)
					continue
				}
				admitted.Add(1)
				mu.Lock()
				now++
				perKey[key]++
				mostAll = max(mostAll, now)
				mostKey = max(mostKey, perKey[key])
				mu.Unlock()
				time.Sleep(500 * time.Microsecond)
				mu.Lock()
				now--
				perKey[key]--
				mu.Unlock()
				leave()
				leave() // a second call does nothing
			}
		}()
	}
	close(start)
	wg.Wait()

	if mostAll > uploadSlots {
		t.Errorf("%d inside at once, want at most %d", mostAll, uploadSlots)
	}
	if mostKey > uploadNetworkSlots {
		t.Errorf("%d from one network at once, want at most %d", mostKey, uploadNetworkSlots)
	}
	if admitted.Load() == 0 || refused.Load() == 0 {
		t.Errorf("admitted %d, refused %d: the test did not press on the bound", admitted.Load(), refused.Load())
	}
	if n, keys := held(g); n != 0 || keys != 0 {
		t.Errorf("at the end %d inside and %d keys, want none", n, keys)
	}
}
