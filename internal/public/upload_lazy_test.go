package public

import (
	"bytes"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/domain"
)

// messageBody is a multipart submission without a file, the shape the contact
// form posts whenever its attachment field is switched on: the fields, and the
// empty file input a browser sends as a part with filename="".
func messageBody(t *testing.T) ([]byte, string) {
	t.Helper()
	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	for _, p := range []struct{ disposition, content string }{
		{`form-data; name="name"`, "Anna"},
		{`form-data; name="anhang"; filename=""`, ""},
		{`form-data; name="nachricht"`, "Hallo"},
	} {
		h := textproto.MIMEHeader{}
		h.Set("Content-Disposition", p.disposition)
		if bytes.Contains([]byte(p.disposition), []byte("filename")) {
			h.Set("Content-Type", "application/octet-stream")
		}
		part, err := w.CreatePart(h)
		if err != nil {
			t.Fatal(err)
		}
		part.Write([]byte(p.content))
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	return buf.Bytes(), w.FormDataContentType()
}

// submitMessage takes messageBody from the request shape prepare makes.
func submitMessage(t *testing.T, h *Handler, ws *domain.Website, prepare func(*http.Request)) (fields string, keeper *attachmentKeeper, refused *uploadRefusal) {
	t.Helper()
	body, ctype := messageBody(t)
	fields, keeper, took, refused := takeBody(t, h, ws, body, ctype, prepare)
	if took == (refused != nil) {
		t.Fatalf("took=%v refused=%+v: one of the two, not both", took, refused)
	}
	return fields, keeper, refused
}

// The gate and the hourly budget count submissions that carry a file, and only
// those (WR-01 of the phase 38 review).
//
// Both were entered for every multipart submission to a form with
// attachments, before anything was known about its parts. The contact form is
// multipart whenever its attachment field is on, so four slow senders closed
// the whole form with 503 on every website of the installation, and a network
// that sent thirty messages without a file in an hour was refused its
// thirty-first. A message without a file costs no spool and is now neither
// counted nor refused by them.
func TestAMessageWithoutAFileIsNeverRefusedByTheUploadBounds(t *testing.T) {
	t.Run("the server is full", func(t *testing.T) {
		h, ws := behindAProxy(t)
		leaves := fillTheServer(t, h.uploads)
		defer func() {
			for _, leave := range leaves {
				leave()
			}
		}()
		fields, _, refused := submitMessage(t, h, ws, via("203.0.113.40"))
		if refused != nil {
			t.Fatalf("a message without a file on a full server: refused=%+v, want taken", refused)
		}
		if fields != "anhang=&nachricht=Hallo&name=Anna" {
			t.Errorf("fields = %q", fields)
		}
		if n, _ := held(h.uploads); n != uploadSlots {
			t.Errorf("%d inside, want the %d fillers only: the message took a slot", n, uploadSlots)
		}
	})

	t.Run("one network, many messages", func(t *testing.T) {
		h, ws := behindAProxy(t)
		var open []*attachmentKeeper
		for i := 1; i <= uploadNetworkPerHour+10; i++ {
			_, keeper, refused := submitMessage(t, h, ws, via("203.0.113.41"))
			if refused != nil {
				t.Fatalf("message %d without a file from one network: refused=%+v", i, refused)
			}
			// Left open: messages inside at once do not count either.
			open = append(open, keeper)
		}
		if n, keys := held(h.uploads); n != 0 || keys != 0 {
			t.Errorf("%d inside and %d network keys for messages without files, want none", n, keys)
		}
		// The network's budget is untouched: its submissions with files are
		// still taken, up to the bound.
		for i := 1; i <= uploadNetworkPerHour; i++ {
			keeper, refused := submitFiles(t, h, ws, via("203.0.113.41"))
			if refused != nil {
				t.Fatalf("submission %d with a file after the messages: refused=%+v: they were counted", i, refused)
			}
			keeper.Close()
		}
		for _, k := range open {
			k.Close()
		}
	})
}

// A submission whose file is refused unread (more files than allowed) uses the
// slot its first file took; one whose fields come first enters the gate at its
// first file and still gives the slot back when it is closed.
func TestTheSlotIsTakenAtTheFirstFile(t *testing.T) {
	h, ws := behindAProxy(t)
	body, ctype := rawMultipart(t, false,
		fld("name", "Anna"),
		filePart("anhang", "a.png", bigPNG(1024)),
		filePart("anhang", "b.png", bigPNG(1024)))
	var insideAtParts []int
	attachPartHook = func() {
		n, _ := held(h.uploads)
		insideAtParts = append(insideAtParts, n)
	}
	t.Cleanup(func() { attachPartHook = nil })
	_, keeper, took, refused := takeBody(t, h, ws, body, ctype, via("203.0.113.42"))
	if !took || refused != nil {
		t.Fatalf("took=%v refused=%+v", took, refused)
	}
	if want := []int{0, 1, 1}; len(insideAtParts) != 3 || insideAtParts[0] != want[0] || insideAtParts[1] != want[1] || insideAtParts[2] != want[2] {
		t.Errorf("inside after each part: %v, want %v", insideAtParts, want)
	}
	keeper.Close()
	if n, keys := held(h.uploads); n != 0 || keys != 0 {
		t.Errorf("after Close %d inside and %d keys, want none", n, keys)
	}
}
