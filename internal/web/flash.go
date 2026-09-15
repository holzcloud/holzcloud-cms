package web

import (
	"context"

	"github.com/alexedwards/scs/v2"

	"github.com/holzcloud/holzcloud-cms/internal/auth"
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

// The keys come from internal/auth, which owns the session, and are not spelled
// out again here.
//
// They were, until v2.4. auth.SessionKeyFlashError and
// auth.SessionKeyFlashSuccess existed and these six calls wrote the strings
// instead, so there were two sources for one key and nothing holding them
// together: renaming the constant would have changed nothing and broken
// nothing, and the flash would simply have stopped appearing. Found by
// `tools/surface`, which noticed that one of the two constants was used
// nowhere at all — the other was, which is how a pair like this survives a
// reading.

// SetFlashError stores an error flash message in the session.
func SetFlashError(sm *scs.SessionManager, ctx context.Context, msg string) {
	sm.Put(ctx, auth.SessionKeyFlashError, i18n.T(i18n.Lang(ctx), msg))
}

// SetFlashSuccess stores a success flash message in the session.
func SetFlashSuccess(sm *scs.SessionManager, ctx context.Context, msg string) {
	sm.Put(ctx, auth.SessionKeyFlashSuccess, i18n.T(i18n.Lang(ctx), msg))
}

// SetFlashWarning stores a warning flash message in the session.
func SetFlashWarning(sm *scs.SessionManager, ctx context.Context, msg string) {
	sm.Put(ctx, auth.SessionKeyFlashWarning, i18n.T(i18n.Lang(ctx), msg))
}

// GetFlash pops all flash keys from the session and returns them.
func GetFlash(sm *scs.SessionManager, ctx context.Context) Flash {
	return Flash{
		Error:   sm.PopString(ctx, auth.SessionKeyFlashError),
		Success: sm.PopString(ctx, auth.SessionKeyFlashSuccess),
		Warning: sm.PopString(ctx, auth.SessionKeyFlashWarning),
	}
}
