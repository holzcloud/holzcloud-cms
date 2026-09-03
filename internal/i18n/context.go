package i18n

import (
	"context"
	"net/http"
)

// The language of the request, so anything that produces text for a person can
// reach it without every function growing a parameter.
//
// It is set once, by the middleware, from the signed-in user's setting or —
// before anybody has signed in — from what the browser asks for.

type langKey struct{}

// WithLang returns a context carrying the language of this request.
func WithLang(ctx context.Context, lang string) context.Context {
	return context.WithValue(ctx, langKey{}, lang)
}

// Lang is the language of this request, German when nothing was set.
func Lang(ctx context.Context) string {
	lang, _ := ctx.Value(langKey{}).(string)
	if lang == "" {
		return Source
	}
	return lang
}

// Middleware puts the language into the context of every request.
//
// choose is asked for the signed-in user's own setting; it returns the empty
// string when nobody is signed in or the user has expressed no preference, and
// the browser's wish is used instead. That order is deliberate: a person who
// has chosen a language in their account means it on every machine, including
// the one whose browser is set to something else.
func Middleware(choose func(*http.Request) string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			lang := ""
			if choose != nil {
				lang = choose(r)
			}
			if !Known(lang) {
				lang = FromRequest(r)
			}
			next.ServeHTTP(w, r.WithContext(WithLang(r.Context(), lang)))
		})
	}
}
