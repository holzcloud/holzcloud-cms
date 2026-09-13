package web

import (
	"errors"
	"net/http"

	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The sentence a person reads when a file is not taken.
//
// It lives here and not in internal/media because media has no reader: the same
// refusal is read by an operator at the administration's upload and by a
// visitor attaching a file to a form, in two different languages. What media
// gives out is a code; this turns it into a sentence for whoever is asking.

// MediaRefusal says, in the language of this request, why a file was not taken.
//
// An error that is not a media.Refusal comes back as the general sentence: it
// is a failure of something else — a disk, a database — and telling somebody
// the file type was wrong would be worse than saying nothing useful.
func MediaRefusal(r *http.Request, err error) string {
	var ref media.Refusal
	if !errors.As(err, &ref) {
		return T(r, "The file could not be accepted.")
	}
	switch ref.Code {
	case media.RefusedKind:
		return Titlef(r, "File type not allowed: %s", ref.Arg)
	case media.RefusedSize:
		return Titlef(r, "The file is larger than allowed (%d MB)", ref.Arg)
	case media.RefusedSVGRef:
		return Titlef(r,
			"The SVG file loads %s from somebody else's server; that is not allowed", ref.Arg)
	default:
		return T(r, "The file could not be read.")
	}
}
