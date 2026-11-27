package media

import (
	"bytes"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/tmplmgr"
)

// SpoolDirName is the directory under the data directory in which a public
// form's files wait while the form is being taken in.
//
// A dot-directory beside media/, so the public media route can never serve it,
// and inside the data directory rather than the system's temporary one because
// the service runs with PrivateTmp and ProtectSystem=strict: a private /tmp may
// be a small RAM-backed tmpfs, and streaming a video there would fill memory
// again, which is the thing the spool exists to avoid.
const SpoolDirName = ".upload-spool"

// SpoolRoot is the spool directory of one installation.
func SpoolRoot(dataDir string) string {
	return filepath.Join(dataDir, SpoolDirName)
}

// ErrSpool marks a spool that could not be created or written, so a caller can
// tell a full or unwritable disk (the server's trouble) from a body that was cut
// short or a file that is too large (the sender's).
var ErrSpool = errors.New("media: the upload spool is not writable")

// spoolWriter marks the write errors of the disk with ErrSpool, so that after
// io.Copy a failed read of the part and a failed write of the file are told
// apart: the copy reports whichever came first and nothing else says which.
type spoolWriter struct{ f *os.File }

func (w spoolWriter) Write(p []byte) (int, error) {
	n, err := w.f.Write(p)
	if err != nil {
		err = fmt.Errorf("%w: %w", ErrSpool, err)
	}
	return n, err
}

// CheckStream is Check for a file that is not to be held in memory: the part of
// a public form, which comes from a stranger.
//
// Check reads the whole file into a slice, which is right for the signed-in
// operator and wrong for anybody else: a few parallel requests with large parts
// would exhaust a small node before the form's spam traps ran. CheckStream
// decides the kind from the first 512 bytes, which fixes the limit that
// applies, and then copies the rest into a file in dir through a reader that
// stops one byte past that limit. Nothing beyond the head and a copy buffer is
// held.
//
// A refusal is a Refusal, as with Check. Any other error is the part's own
// read error (a body cut short, a MaxBytesError) or an ErrSpool, both wrapped
// with %w so errors.Is and errors.As still see them; on every error the file
// is already gone.
//
// The Intake it returns has no content in memory: Oversize, Scaled and the
// staging read content, and must never be given a spooled intake. Only KeepFor
// knows to open its file. Discard removes the file again when the caller has
// decided against it, or after KeepFor.
func CheckStream(part io.Reader, filename string, limits Limits, dir string) (*Intake, error) {
	// The head by a loop of Reads, not io.ReadFull: that turns a short reader's
	// io.EOF into io.ErrUnexpectedEOF, which is also what a cut multipart part
	// returns, and the two must stay different.
	head := make([]byte, 0, 512)
	for len(head) < cap(head) {
		n, err := part.Read(head[len(head):cap(head)])
		head = head[:len(head)+n]
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("media: read the head of a part: %w", err)
		}
	}

	// The kind comes from the bytes and never from the Content-Type the sender
	// wrote, and it comes before the limit is chosen.
	mimeType, err := ValidateMIME(bytes.NewReader(head), filename)
	if err != nil {
		return nil, Refusal{Code: RefusedKind, Arg: err.Error()}
	}
	limit := limits.For(mimeType)

	f, err := os.CreateTemp(dir, "part-*")
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrSpool, err)
	}
	path := f.Name()
	fail := func(err error) (*Intake, error) {
		f.Close()
		os.Remove(path)
		return nil, err
	}

	n, err := io.Copy(spoolWriter{f}, io.LimitReader(io.MultiReader(bytes.NewReader(head), part), limit+1))
	if err != nil {
		return fail(fmt.Errorf("media: copy a part: %w", err))
	}
	if n > limit {
		return fail(Refusal{Code: RefusedSize, Arg: limit >> 20})
	}

	// An SVG is markup and can pull a stylesheet or an image from another
	// server. The same scanner as in Check runs, over the file just written and
	// bounded by the limit; this is the one kind that is read back whole.
	if mimeType == "image/svg+xml" {
		if _, err := f.Seek(0, io.SeekStart); err != nil {
			return fail(Refusal{Code: RefusedRead})
		}
		content, err := io.ReadAll(io.LimitReader(f, limit+1))
		if err != nil {
			return fail(Refusal{Code: RefusedRead})
		}
		if refs := tmplmgr.CheckExternalRefs("upload.svg", string(content)); len(refs) > 0 {
			return fail(Refusal{Code: RefusedSVGRef, Arg: refs[0].URL})
		}
	}
	if err := f.Close(); err != nil {
		os.Remove(path)
		return nil, fmt.Errorf("%w: %w", ErrSpool, err)
	}

	return &Intake{
		MimeType:     mimeType,
		OriginalName: filename,
		Filename:     GenerateFilename(filename),
		Size:         n,
		limit:        limit,
		spool:        path,
	}, nil
}

// Discard removes the spool file of an intake made by CheckStream, if there is
// one. It is safe to call twice, and on an intake that was never spooled.
func (in *Intake) Discard() {
	if in.spool == "" {
		return
	}
	os.Remove(in.spool)
	in.spool = ""
}

// SweepSpool removes what lies in the spool directory and is older than
// olderThan, and says how many entries went. A missing directory is nothing to
// sweep.
//
// At start nothing can be in flight, so the caller passes 0 and everything
// goes. While the server runs, a request's directory lives at most as long as
// its request (the read timeout is 120 seconds), so one that is an hour old is
// the leftover of a request that never returned.
func SweepSpool(dataDir string, olderThan time.Duration) (int, error) {
	root := SpoolRoot(dataDir)
	entries, err := os.ReadDir(root)
	if errors.Is(err, os.ErrNotExist) {
		return 0, nil
	}
	if err != nil {
		return 0, fmt.Errorf("sweep upload spool: %w", err)
	}
	cutoff := time.Now().Add(-olderThan)
	removed := 0
	for _, e := range entries {
		info, err := e.Info()
		if err != nil || info.ModTime().After(cutoff) {
			continue
		}
		if err := os.RemoveAll(filepath.Join(root, e.Name())); err == nil {
			removed++
		}
	}
	return removed, nil
}
