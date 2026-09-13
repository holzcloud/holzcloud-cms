package public

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/url"
	"strings"

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
	dataDir   string
	store     *media.Store

	listed []plugin.AttachedFile
	// intakes are the checked files, in the same order as listed minus the
	// refused ones.
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

func (k *attachmentKeeper) List() []plugin.AttachedFile { return k.listed }

func (k *attachmentKeeper) Keep(ctx context.Context) ([]plugin.KeptFile, error) {
	if k.done {
		return k.kept, nil
	}
	k.done = true
	for _, in := range k.intakes {
		m, existed, err := in.intake.Keep(ctx, k.store, k.websiteID, k.dataDir)
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

// takeAttachments parses a multipart submission for a plugin that may have one.
//
// It returns the fields as an ordinary encoded form and a keeper for the files.
// A request that is not multipart, or a plugin without the permission, gets
// nothing here and reads its body as before.
func (h *Handler) takeAttachments(r *http.Request, websiteID int64, allowed bool) (body string, keeper *attachmentKeeper, ok bool) {
	if !allowed || h.mediaStore == nil {
		return "", nil, false
	}
	kind, params, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || !strings.HasPrefix(kind, "multipart/form-data") || params["boundary"] == "" {
		return "", nil, false
	}

	limits := media.Limits{Image: h.maxMediaSize, Video: h.maxVideoSize}
	// The outer bound on the whole request, so a sender cannot make the host
	// read for ever: the largest single file, times the number allowed, plus
	// room for the fields.
	outer := limits.Video*MaxAttachments + int64(MaxPluginBodyBytes)
	reader := multipart.NewReader(http.MaxBytesReader(nil, r.Body, outer), params["boundary"])

	fields := url.Values{}
	keeper = &attachmentKeeper{websiteID: websiteID, dataDir: h.dataDir, store: h.mediaStore}

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
				Refused: fmt.Sprintf("at most %d files", MaxAttachments),
			})
			part.Close()
			continue
		}

		// media.Check wants a multipart.File, which is what a ReadSeeker over
		// the part gives it; the part itself cannot seek, and every check here
		// needs to read the head twice.
		file, header, err := bufferPart(part)
		part.Close()
		if err != nil {
			keeper.listed = append(keeper.listed, plugin.AttachedFile{
				Field: name, Name: part.FileName(), Refused: "the file could not be read",
			})
			continue
		}
		in, err := media.Check(file, header, limits)
		if err != nil {
			// In the language of the PAGE, not of the request: this is read by
			// a visitor, and the plugin cannot look it up itself — media.Check
			// gives out a code precisely so each caller answers its own reader.
			keeper.listed = append(keeper.listed, plugin.AttachedFile{
				Field: name, Name: header.Filename,
				Refused: refusalText(r.Context(), err),
			})
			continue
		}
		keeper.intakes = append(keeper.intakes, keptIntake{field: name, intake: in})
		keeper.listed = append(keeper.listed, plugin.AttachedFile{
			Field: name, Name: in.OriginalName, MimeType: in.MimeType, Size: in.Size,
		})
	}
	return fields.Encode(), keeper, true
}

// bufferPart reads one part into memory so it can be read twice.
//
// Every check media.Check makes needs the head of the file and then the whole
// of it: the magic bytes, then the SVG scan, then the bytes themselves. A
// multipart part is a stream and cannot be rewound, so it is buffered — bounded
// by the same reader that bounds the request, which is what stops this being
// the memory hole the 256 KB body limit exists to prevent.
func bufferPart(part *multipart.Part) (multipart.File, *multipart.FileHeader, error) {
	var buf bytes.Buffer
	n, err := io.Copy(&buf, part)
	if err != nil {
		return nil, nil, err
	}
	return sectionFile{bytes.NewReader(buf.Bytes())},
		&multipart.FileHeader{Filename: part.FileName(), Size: n}, nil
}

// sectionFile is a bytes.Reader wearing the multipart.File interface. Close is
// the only method it has to invent, and there is nothing to close.
type sectionFile struct{ *bytes.Reader }

func (sectionFile) Close() error { return nil }

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
