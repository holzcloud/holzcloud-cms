package public

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
	"github.com/holzcloud/holzcloud-cms/internal/media"
	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// An attachment on a plugin's form.
//
// The file never reaches the plugin. It could not: a module that pulled five
// megabytes into its linear memory would be a way to exhaust a small node with
// one request, which is the reason MaxPluginBodyBytes is 256 KB and stays 256
// KB. So the host parses the multipart, hands the plugin the ordinary fields —
// so a plugin written before attachments existed reads exactly what it read
// before — and holds the files until the plugin says to keep them.
//
// Holding rather than storing is the other half. A contact form has to run its
// spam traps on the fields BEFORE anything reaches the disk, or a robot with a
// five-megabyte attachment fills the disk whatever the traps decide. The plugin
// refuses the submission, never calls files.keep, and nothing was written.

// MaxAttachments bounds how many files one submission may bring.
//
// Three, because a form that wants more is a form that wants a folder, and a
// folder is what a link to a file service is for. It also bounds the memory one
// request can hold: three times the image limit and no more.
const MaxAttachments = 3

// attachmentKeeper holds one request's files and knows how to store them.
type attachmentKeeper struct {
	websiteID int64
	// pluginID is the plugin whose form took the files: they are stored as
	// its own, and only its privacy answer can delete them.
	pluginID string
	dataDir  string
	store    *media.Store

	// mu guards the state below between the request, the plugin's Keep and
	// the keep's own goroutine: a keep that is reading a spool file while
	// Close removes the directory would store half a file.
	mu sync.Mutex
	// spoolDir is this request's own directory below the spool root, made when
	// the first file part arrives ("" until then). Close removes it.
	spoolDir string
	closed   bool
	// leave gives back the submission's slot in the upload gate (upload_gate.go).
	// It lives here so that the one thing that ends a keeper, Close, also ends
	// the slot: no exit path has a release of its own to forget.
	leave func()

	listed []plugin.AttachedFile
	// intakes are the checked files, in the same order as listed minus the
	// refused ones. Each waits in a spool file; none is held in memory.
	intakes []keptIntake
	// kept is filled by the first Keep and answered again by any second one:
	// a plugin that asks twice gets the same ids and stores nothing further.
	kept []plugin.KeptFile
	done bool
	// keeping is set while keepAll runs; a Close in that time leaves the
	// spool and the slot to it. late is set when Keep answered the plugin
	// before keepAll was done: what it stores after that is taken away again.
	keeping bool
	late    bool
}

type keptIntake struct {
	field  string
	intake *media.Intake
}

// plugin.FileKeeper keeps exactly its two methods, so that the plugin runtime
// of phase 36 and the SDK stay as they are. Closing is not the plugin's
// business: the middleware, which holds the concrete keeper, closes it when
// the request is answered, and takeAttachments closes it itself on every path
// that does not hand it on.
var _ io.Closer = (*attachmentKeeper)(nil)

func (k *attachmentKeeper) List() []plugin.AttachedFile { return k.listed }

// Keep stores the files and answers the plugin with their ids.
//
// It runs inside the plugin's call, whose deadline is CallTimeout and whose
// module the engine closes when that passes; a keep that overran it used to
// fail its database work, then the whole call, and the visitor got a 404 for
// a lost message (WR-03 of the phase 38 review). So the disk and database work
// runs in a goroutine of its own, detached from the call's deadline and
// bounded by keepBound, and Keep waits for it only until keepMargin before the
// call's deadline. If it is done by then the plugin gets every file. If not,
// the plugin gets what was stored so far and goes on to store the message
// without the rest; the keep stops after the file it is writing, takes that
// one away again, and the operator's log says so. A message without its
// attachment is worse than a message with it and far better than none.
//
// The call's own time cannot be stopped while the host works: that would be a
// change to the plugin runtime for one host function. This keeps the runtime
// as it is.
func (k *attachmentKeeper) Keep(ctx context.Context) ([]plugin.KeptFile, error) {
	k.mu.Lock()
	if k.done || k.closed {
		// After Close the spool is gone and nothing may be stored from it: a
		// call that arrives late answers what it had kept, which is nothing
		// when it never ran.
		defer k.mu.Unlock()
		return k.kept, nil
	}
	k.done = true
	answerBy, bounded := ctx.Deadline()
	if bounded {
		answerBy = answerBy.Add(-keepMargin)
		if !time.Now().Before(answerBy) {
			// Not even the time to start: the plugin goes on without files.
			k.late = true
			n := len(k.intakes)
			k.mu.Unlock()
			k.logLate(n)
			return nil, nil
		}
	}
	k.keeping = true
	finished := make(chan struct{})
	k.mu.Unlock()

	go k.keepAll(context.WithoutCancel(ctx), finished)

	if bounded {
		timer := time.NewTimer(time.Until(answerBy))
		defer timer.Stop()
		select {
		case <-finished:
		case <-timer.C:
			k.mu.Lock()
			k.late = true
			kept := append([]plugin.KeptFile(nil), k.kept...)
			n := len(k.intakes) - len(kept)
			k.mu.Unlock()
			k.logLate(n)
			return kept, nil
		}
	} else {
		<-finished
	}
	k.mu.Lock()
	defer k.mu.Unlock()
	return k.kept, nil
}

// keepAll stores the files one after the other, until it is done or Keep has
// answered without it, and then hands the spool and the slot to a Close that
// came in the meantime. It closes finished when it is over.
func (k *attachmentKeeper) keepAll(ctx context.Context, finished chan struct{}) {
	ctx, cancel := context.WithTimeout(ctx, keepBound)
	defer cancel()
	defer func() {
		// The goroutine is the keeper's own and no caller can recover for
		// it: a panic in a strip of a stranger's file must not end the
		// process.
		if p := recover(); p != nil {
			slog.Error("public form attachment: the keep panicked", "plugin", k.pluginID, "panic", p)
		}
		k.mu.Lock()
		k.keeping = false
		if k.closed {
			k.releaseLocked()
		}
		k.mu.Unlock()
		close(finished)
	}()

	for _, in := range k.intakes {
		k.mu.Lock()
		late := k.late
		k.mu.Unlock()
		if late {
			return
		}
		if keepHook != nil {
			keepHook()
		}
		m, existed, err := in.intake.KeepFor(ctx, k.store, k.websiteID, k.pluginID, k.dataDir)
		if err != nil {
			// One file that cannot be written does not lose the others, and
			// does not lose the message either: the plugin has already decided
			// this submission is real.
			continue
		}
		// The spool file goes as soon as its copy is stored, so a submission
		// holds a file twice on the disk for no longer than one copy takes.
		in.intake.Discard()
		k.mu.Lock()
		if k.late {
			k.mu.Unlock()
			// Keep has answered without it, so no message names this file:
			// it goes again, unless it was a file already stored before.
			if !existed {
				if err := k.store.DeleteOwned(ctx, k.websiteID, k.pluginID, m.ID, k.dataDir); err != nil {
					slog.Error("public form attachment: a file kept too late could not be removed",
						"plugin", k.pluginID, "media", m.ID, "error", err)
				}
			}
			return
		}
		k.kept = append(k.kept, plugin.KeptFile{
			Field: in.field, MediaID: m.ID, Name: m.OriginalName,
			Filename: m.Filename, Existed: existed,
		})
		k.mu.Unlock()
	}
}

// logLate tells the operator that a message went on without n of its files.
func (k *attachmentKeeper) logLate(n int) {
	slog.Warn("public form attachment: not kept within the plugin's call; the message goes on without it",
		"plugin", k.pluginID, "website", k.websiteID, "files", n)
}

// Close removes the request's spool directory and everything in it and gives
// the submission's slot back, once; any later call does nothing, and no
// directory is fine. The slot goes back after the files are gone, so a slot
// that is free is also a slot whose disk is.
func (k *attachmentKeeper) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return nil
	}
	k.closed = true
	if k.keeping {
		// A keep is still writing from the spool: it removes the spool and
		// gives the slot back itself when it ends (keepAll), so the disk it
		// is filling stays counted until then.
		return nil
	}
	return k.releaseLocked()
}

// releaseLocked removes the spool and gives the slot back. The caller holds
// k.mu and has set closed.
func (k *attachmentKeeper) releaseLocked() error {
	var err error
	if k.spoolDir != "" {
		err = os.RemoveAll(k.spoolDir)
	}
	if k.leave != nil {
		k.leave()
	}
	return err
}

// uploadRefusal is a submission refused as a whole, before the plugin was
// called: the status to answer and, for a refusal that comes back later, how
// long to wait. The visitor gets the standard status text, as for a plugin
// that is busy.
type uploadRefusal struct {
	Status     int
	RetryAfter time.Duration
}

// The bounds of one submission. Each number is the least that lets an honest
// form through; everything above it is a body nobody filled in by hand.
const (
	// maxFormParts counts every part, named or not, file or field. A contact
	// form of twenty assembled fields with its hidden fields and three files
	// needs about thirty; more than twice that is a sender probing.
	maxFormParts = 64
	// multipartSlack is room for the boundaries and part headers on top of the
	// content the other bounds count.
	multipartSlack = 64 << 10
)

// attachmentBound is the most one submission may weigh: the largest single
// file, two images, the field text and the framing.
//
// At most one part may use the video limit (owner decision D2), so the largest
// honest submission is one video and two images plus the text. The bound that
// stood here before counted three videos, nearly 192 MB at the default limits.
func attachmentBound(l media.Limits) int64 {
	return max(l.Video, l.Image) + (MaxAttachments-1)*l.Image + MaxPluginBodyBytes + multipartSlack
}

// attachPartHook exists for the test that proves a panic inside the take
// removes the spool, and for nothing else. It is nil in production and is
// called once per part after the part was handled.
var attachPartHook func()

// keepHook exists for the test of a keep that overruns the plugin's call, and
// for nothing else. It is nil in production and is called before each file is
// kept.
var keepHook func()

// How long a keep may take. keepMargin is the part of the plugin's call a keep
// leaves to the plugin, to store the message and answer, when it has to answer
// before it is done (Keep). keepBound bounds the keep's own disk and database
// work, which runs outside the call's deadline: a 64 MB video is written and
// hashed in about a second on a small node, so a minute is a hang and not a
// slow disk.
const (
	keepMargin = plugin.CallTimeout / 4
	keepBound  = time.Minute
)

// whole is the refusal of a submission as a whole for the error that ended it.
//
// A body over the bound, or a part header over its own, is 413. Any other read
// error is 400: a body cut short is not a shorter form, and a stored partial
// file would look like a finished upload.
func whole(err error) *uploadRefusal {
	var tooBig *http.MaxBytesError
	if errors.As(err, &tooBig) || errors.Is(err, errPartHeaderTooLarge) {
		return &uploadRefusal{Status: http.StatusRequestEntityTooLarge}
	}
	return &uploadRefusal{Status: http.StatusBadRequest}
}

// takeAttachments parses a multipart submission for a plugin that may have one.
//
// It returns the fields as an ordinary encoded form and a keeper for the files.
// A request that is not multipart, or a plugin without the permission, gets
// nothing here and reads its body as before. A submission that is refused as a
// whole comes back as refused, with no keeper and with ok false: the plugin is
// not called and nothing stays in the spool.
//
// Each file part is checked and copied to the request's spool as it arrives,
// no further than its kind's limit and one byte; no part is held in memory.
// The keeper is the caller's to Close once it is handed on; every return
// before that closes it here.
func (h *Handler) takeAttachments(w http.ResponseWriter, r *http.Request, websiteID int64, pluginID string, allowed bool) (body string, keeper *attachmentKeeper, ok bool, refused *uploadRefusal) {
	if !allowed || h.mediaStore == nil {
		return "", nil, false, nil
	}
	kind, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(kind, "multipart/form-data") || params["boundary"] == "" {
		return "", nil, false, nil
	}

	limits := media.Limits{Image: h.maxMediaSize, Video: h.maxVideoSize}
	bound := attachmentBound(limits)
	// A body that says it is over the bound is refused before a byte of it is
	// read, and before it takes a slot: it costs nothing and counts for
	// nothing. A chunked one (-1) says nothing and goes on to the reader.
	if r.ContentLength > bound {
		return "", nil, false, &uploadRefusal{Status: http.StatusRequestEntityTooLarge}
	}

	// A network that names a visitor shares a bound with that network only. An
	// address that names none (loopback, private, link-local: a proxy that
	// HOLZCLOUD_TRUSTED_PROXIES does not cover) meets the server bound alone:
	// every visitor would share it, and one stranger would close the form for
	// everyone. ThrottleKey writes the once-only warning that names the
	// remedy.
	network := ""
	if key, public := h.clientIP.ThrottleKey(r); public {
		network = key
	}
	// The keeper is made at once, so that the deferred Close below removes the
	// spool and gives back the slot, once the first file took one, on every
	// path that does not hand the keeper on: a refusal, an early return, a
	// panic. No path releases either by hand.
	keeper = &attachmentKeeper{websiteID: websiteID, pluginID: pluginID, dataDir: h.dataDir, store: h.mediaStore}
	// The deferred call holds its own copy of the keeper: the named result is
	// set to nil by the return of a refusal before the defer runs.
	own := keeper
	handedOn := false
	defer func() {
		if !handedOn {
			own.Close()
		}
	}()

	// The gate, the network's hourly budget and the free-space floor are asked
	// at the first file part and not here (WR-01 of the phase 38 review). A
	// form with attachments is multipart whether or not the visitor chose a
	// file, and a message without one costs no spool: counting it would let
	// four slow senders close every form of the installation, and a network
	// spend its hour on messages. So a submission enters at its first file,
	// before a byte of that file is read, and one without a file never does.
	var (
		admitted     bool
		filesRefused bool
		freeNow      uint64
		freeErr      error
		floor        uint64
		warned       bool
	)
	admit := func() *uploadRefusal {
		if admitted {
			return nil
		}
		admitted = true
		// While the disk is low a file is refused and the message is not, and
		// nothing is spooled, so there is nothing for the gate to count. A
		// full disk is exactly when the form is still wanted; sessions are
		// written to the same database, and a spool that filled the disk
		// would stop everybody signing in. An unreadable free space counts as
		// low.
		freeNow, freeErr = h.freeSpace()
		floor = h.AttachmentFloor()
		if freeErr != nil || freeNow < floor {
			filesRefused = true
			return nil
		}
		// A full server or network answers at once, and the file is not read.
		leave, status := h.uploads.enter(network)
		if leave == nil {
			return &uploadRefusal{Status: status, RetryAfter: uploadRetryAfter}
		}
		// The keeper carries the slot from here on: its Close gives it back.
		// Set without the lock: the take runs on one goroutine, and nothing
		// else holds the keeper before it is handed on.
		keeper.leave = leave
		// The network's hourly budget, once the slot is held. A server that
		// was full reserved nothing. The attempt is settled as spent at once
		// and never refunded: the bytes are read whatever the plugin decides,
		// and a sender who keeps being refused is still sending.
		if network != "" {
			attempt, wait := h.uploadNets.Reserve(network, "")
			if attempt == nil {
				return &uploadRefusal{Status: http.StatusTooManyRequests, RetryAfter: max(wait, uploadRetryAfter)}
			}
			attempt.Fail()
		}
		return nil
	}

	// With the ResponseWriter, so the server closes the connection after an
	// overflow instead of reading on. The header bound sits between the two:
	// mime/multipart would hold a part's whole header in memory before this
	// code sees the part (partheader.go).
	reader := multipart.NewReader(
		newPartHeaderBound(http.MaxBytesReader(w, r.Body, bound), params["boundary"], maxPartHeaderBytes),
		params["boundary"])
	fields := url.Values{}
	lang := i18n.Lang(r.Context())

	var (
		parts      int
		fileParts  int
		fieldBytes int
		// videoTaken is set once a part was accepted under the video limit:
		// from then on every part is held to the image limit (D2).
		videoTaken bool
	)
	// A part is not closed here: NextPart closes the one before it, which
	// reads what is left of it, and a submission refused whole is not read
	// any further at all.
	one := func(part *multipart.Part) *uploadRefusal {
		name := part.FormName()
		if name == "" {
			return nil
		}
		filename := part.FileName()
		if filename == "" {
			// An ordinary field. The text of all of them together is bounded
			// by what one body was bounded by before attachments existed.
			remaining := MaxPluginBodyBytes - fieldBytes
			text, err := io.ReadAll(io.LimitReader(part, int64(remaining)+1))
			if err != nil {
				return whole(err)
			}
			if len(text) > remaining {
				return &uploadRefusal{Status: http.StatusRequestEntityTooLarge}
			}
			fieldBytes += len(text)
			fields.Add(name, string(text))
			return nil
		}

		fileParts++
		if fileParts > MaxAttachments {
			keeper.listed = append(keeper.listed, plugin.AttachedFile{
				Field: name, Name: filename,
				Refused: i18n.Tf(lang, "at most %d files", MaxAttachments),
			})
			return nil
		}

		if refusal := admit(); refusal != nil {
			return refusal
		}
		if filesRefused {
			// Once per submission, and only when a file is turned away: a
			// message without one is not the operator's concern.
			if !warned {
				warned = true
				slog.Warn("a public form attachment is refused: the data directory is below its free-space floor",
					"free_bytes", freeNow, "floor_bytes", floor, "error", freeErr)
			}
			// Named and never read: NextPart drains what is left of the part.
			keeper.listed = append(keeper.listed, plugin.AttachedFile{
				Field: name, Name: filename,
				Refused: i18n.T(lang, "The file could not be accepted."),
			})
			return nil
		}

		partLimits := limits
		if videoTaken {
			partLimits.Video = min(limits.Video, limits.Image)
		}
		dir, err := h.requestSpool(keeper)
		var in *media.Intake
		if err == nil {
			in, err = media.CheckStream(part, filename, partLimits, dir)
		}
		if err != nil {
			var ref media.Refusal
			switch {
			case errors.As(err, &ref):
				// In the language of the PAGE, not of the request: this is
				// read by a visitor, and the plugin cannot look it up itself —
				// the refusal is a code precisely so each caller answers its
				// own reader.
				keeper.listed = append(keeper.listed, plugin.AttachedFile{
					Field: name, Name: filename, Refused: refusalText(r.Context(), err),
				})
				return nil
			case errors.Is(err, media.ErrSpool):
				slog.Error("upload spool", "error", err)
				keeper.listed = append(keeper.listed, plugin.AttachedFile{
					Field: name, Name: filename,
					Refused: i18n.T(lang, "The file could not be accepted."),
				})
				return nil
			}
			return whole(err)
		}
		if in.MimeType == "video/mp4" && limits.Video > limits.Image {
			videoTaken = true
		}
		keeper.intakes = append(keeper.intakes, keptIntake{field: name, intake: in})
		keeper.listed = append(keeper.listed, plugin.AttachedFile{
			Field: name, Name: in.OriginalName, MimeType: in.MimeType, Size: in.Size,
		})
		return nil
	}

	for {
		part, err := reader.NextPart()
		// Identity, not errors.Is: a body that ends before its closing
		// boundary comes back as a wrapped io.EOF, for which errors.Is is
		// true. Only the bare io.EOF is the end of a finished form.
		if err == io.EOF {
			break
		}
		if err != nil {
			return "", nil, false, whole(err)
		}
		parts++
		if parts > maxFormParts {
			return "", nil, false, &uploadRefusal{Status: http.StatusRequestEntityTooLarge}
		}
		if refusal := one(part); refusal != nil {
			return "", nil, false, refusal
		}
		if attachPartHook != nil {
			attachPartHook()
		}
	}
	handedOn = true
	return fields.Encode(), keeper, true, nil
}

// requestSpool makes the request's own spool directory on first use and says
// where it is.
func (h *Handler) requestSpool(k *attachmentKeeper) (string, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.spoolDir != "" {
		return k.spoolDir, nil
	}
	root := media.SpoolRoot(h.dataDir)
	if err := os.MkdirAll(root, 0o700); err != nil {
		return "", fmt.Errorf("%w: %w", media.ErrSpool, err)
	}
	dir, err := os.MkdirTemp(root, "req-*")
	if err != nil {
		return "", fmt.Errorf("%w: %w", media.ErrSpool, err)
	}
	k.spoolDir = dir
	return dir, nil
}

// refusalText says, in the language this page is published in, why a file was
// not taken.
//
// The twin of web.MediaRefusal, which answers an operator. The two exist
// separately because they answer two different people: LocaleMiddleware put the
// page's language into this context, and the admin's middleware put the
// operator's into the other.
func refusalText(ctx context.Context, err error) string {
	var ref media.Refusal
	if !errors.As(err, &ref) {
		return i18n.T(i18n.Lang(ctx), "The file could not be accepted.")
	}
	lang := i18n.Lang(ctx)
	switch ref.Code {
	case media.RefusedKind:
		return i18n.Tf(lang, "File type not allowed: %s", ref.Arg)
	case media.RefusedSize:
		return i18n.Tf(lang, "The file is larger than allowed (%d MB)", ref.Arg)
	case media.RefusedSVGRef:
		return i18n.Tf(lang,
			"The SVG file loads %s from somebody else's server; that is not allowed", ref.Arg)
	default:
		return i18n.T(lang, "The file could not be read.")
	}
}
