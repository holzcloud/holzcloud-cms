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

	// mu serialises Keep and Close: a Keep that is reading a spool file while
	// Close removes the directory would store half a file.
	mu sync.Mutex
	// spoolDir is this request's own directory below the spool root, made when
	// the first file part arrives ("" until then). Close removes it.
	spoolDir string
	closed   bool

	listed []plugin.AttachedFile
	// intakes are the checked files, in the same order as listed minus the
	// refused ones. Each waits in a spool file; none is held in memory.
	intakes []keptIntake
	// kept is filled by the first Keep and answered again by any second one:
	// a plugin that asks twice gets the same ids and stores nothing further.
	kept []plugin.KeptFile
	done bool
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

func (k *attachmentKeeper) Keep(ctx context.Context) ([]plugin.KeptFile, error) {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.done || k.closed {
		// After Close the spool is gone and nothing may be stored from it: a
		// call that arrives late answers what it had kept, which is nothing
		// when it never ran.
		return k.kept, nil
	}
	k.done = true
	for _, in := range k.intakes {
		m, existed, err := in.intake.KeepFor(ctx, k.store, k.websiteID, k.pluginID, k.dataDir)
		if err != nil {
			// One file that cannot be written does not lose the others, and
			// does not lose the message either: the plugin has already decided
			// this submission is real.
			continue
		}
		k.kept = append(k.kept, plugin.KeptFile{
			Field: in.field, MediaID: m.ID, Name: m.OriginalName,
			Filename: m.Filename, Existed: existed,
		})
	}
	return k.kept, nil
}

// Close removes the request's spool directory and everything in it, once; any
// later call does nothing, and no directory is fine.
func (k *attachmentKeeper) Close() error {
	k.mu.Lock()
	defer k.mu.Unlock()
	if k.closed {
		return nil
	}
	k.closed = true
	if k.spoolDir == "" {
		return nil
	}
	return os.RemoveAll(k.spoolDir)
}

// uploadRefusal is a submission refused as a whole, before the plugin was
// called: the status to answer and, for a refusal that comes back later, how
// long to wait. The visitor gets the standard status text, as for a plugin
// that is busy.
type uploadRefusal struct {
	Status     int
	RetryAfter time.Duration
}

// takeAttachments parses a multipart submission for a plugin that may have one.
//
// It returns the fields as an ordinary encoded form and a keeper for the files.
// A request that is not multipart, or a plugin without the permission, gets
// nothing here and reads its body as before. A submission that is refused as a
// whole comes back as refused, with no keeper.
//
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
	// The outer bound on the whole request, so a sender cannot make the host
	// read for ever: the largest single file, times the number allowed, plus
	// room for the fields. With the ResponseWriter, so the server closes the
	// connection after an overflow.
	outer := limits.Video*MaxAttachments + int64(MaxPluginBodyBytes)
	reader := multipart.NewReader(http.MaxBytesReader(w, r.Body, outer), params["boundary"])

	fields := url.Values{}
	keeper = &attachmentKeeper{websiteID: websiteID, pluginID: pluginID, dataDir: h.dataDir, store: h.mediaStore}
	// Whatever happens below, the spool of this request is removed unless the
	// keeper was handed on: a panic, a refusal or an early return needs no
	// path of its own to remember it.
	handedOn := false
	defer func() {
		if !handedOn {
			keeper.Close()
		}
	}()
	lang := i18n.Lang(r.Context())

	for {
		part, err := reader.NextPart()
		if err != nil {
			break
		}
		name := part.FormName()
		if name == "" {
			part.Close()
			continue
		}
		if part.FileName() == "" {
			// An ordinary field. Bounded like the body it is replacing.
			var sb strings.Builder
			buf := make([]byte, 4096)
			for sb.Len() < MaxPluginBodyBytes {
				n, err := part.Read(buf)
				sb.Write(buf[:n])
				if err != nil {
					break
				}
			}
			fields.Add(name, sb.String())
			part.Close()
			continue
		}

		if len(keeper.intakes) >= MaxAttachments {
			keeper.listed = append(keeper.listed, plugin.AttachedFile{
				Field: name, Name: part.FileName(),
				Refused: i18n.Tf(lang, "at most %d files", MaxAttachments),
			})
			part.Close()
			continue
		}

		dir, err := h.requestSpool(keeper)
		var in *media.Intake
		if err == nil {
			in, err = media.CheckStream(part, part.FileName(), limits, dir)
		}
		filename := part.FileName()
		part.Close()
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
				continue
			case errors.Is(err, media.ErrSpool):
				slog.Error("upload spool", "error", err)
				keeper.listed = append(keeper.listed, plugin.AttachedFile{
					Field: name, Name: filename,
					Refused: i18n.T(lang, "The file could not be accepted."),
				})
				continue
			}
			break
		}
		keeper.intakes = append(keeper.intakes, keptIntake{field: name, intake: in})
		keeper.listed = append(keeper.listed, plugin.AttachedFile{
			Field: name, Name: in.OriginalName, MimeType: in.MimeType, Size: in.Size,
		})
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
