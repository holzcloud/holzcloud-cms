package domain

import "context"

type contextKey struct{ name string }

var websiteKey = &contextKey{"website"}

// WebsiteFromContext retrieves the resolved Website from the request context.
// Returns nil if no website is set.
func WebsiteFromContext(ctx context.Context) *Website {
	ws, _ := ctx.Value(websiteKey).(*Website)
	return ws
}

// WebsiteToContext stores a Website in the request context.
func WebsiteToContext(ctx context.Context, ws *Website) context.Context {
	return context.WithValue(ctx, websiteKey, ws)
}
