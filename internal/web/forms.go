package web

import "net/http"

// FormErrors maps a field name to the message shown next to that field.
type FormErrors map[string]string

// Add records a message for a field, keeping the first one per field so the
// most specific check wins.
func (e FormErrors) Add(field, message string) {
	if e == nil {
		return
	}
	if _, exists := e[field]; !exists {
		e[field] = message
	}
}

// Any reports whether anything failed validation.
func (e FormErrors) Any() bool { return len(e) > 0 }

// FormState is embedded in a page's data struct so a failed submit can be
// re-rendered with what the user typed.
//
// The previous pattern — flash message plus 303 back to an empty form — threw
// the submission away: a long article saved with an empty title was simply gone,
// with a one-line explanation where the text used to be.
type FormState struct {
	Errors FormErrors
	// Conflict carries a message shown above the form as a whole, used for
	// optimistic-locking conflicts where no single field is at fault.
	Conflict string
}

// NewFormState returns a state with an initialised error map.
func NewFormState() FormState { return FormState{Errors: FormErrors{}} }

// RenderFormError renders a page template with 422 Unprocessable Content.
//
// 422 rather than 200 so the response is honestly labelled — htmx swaps it, a
// plain browser shows it, and a caching layer will not store it as a success.
//
// The status has to be written after the headers and before the body, which is
// why this cannot simply delegate to RenderAdmin.
func RenderFormError(w http.ResponseWriter, pt *PageTemplates, r *http.Request, name string, data any) error {
	return RenderAdminStatus(w, pt, r, name, data, http.StatusUnprocessableEntity)
}
