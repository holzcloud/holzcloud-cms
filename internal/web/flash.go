package web

import (
	"context"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/i18n"
)

// A flash is translated when it is put away, not when it is taken out.
//
// It is read on the next request by the same person in the same language, and
// translating here means the 150-odd call sites all over the admin stay plain
// German sentences — nobody has to remember to wrap a message.

// Flash holds flash message data popped from the session.
type Flash struct {
	Error   string
	Success string
	Warning string
}

// SetFlashError stores an error flash message in the session.
func SetFlashError(sm *scs.SessionManager, ctx context.Context, msg string) {
	sm.Put(ctx, "flash_error", i18n.T(i18n.Lang(ctx), msg))
}

// SetFlashSuccess stores a success flash message in the session.
func SetFlashSuccess(sm *scs.SessionManager, ctx context.Context, msg string) {
	sm.Put(ctx, "flash_success", i18n.T(i18n.Lang(ctx), msg))
}

// SetFlashWarning stores a warning flash message in the session.
func SetFlashWarning(sm *scs.SessionManager, ctx context.Context, msg string) {
	sm.Put(ctx, "flash_warning", i18n.T(i18n.Lang(ctx), msg))
}

// GetFlash pops all flash keys from the session and returns them.
func GetFlash(sm *scs.SessionManager, ctx context.Context) Flash {
	return Flash{
		Error:   sm.PopString(ctx, "flash_error"),
		Success: sm.PopString(ctx, "flash_success"),
		Warning: sm.PopString(ctx, "flash_warning"),
	}
}
