package web

import "github.com/holzcloud/holzcloud-cms/internal/localpath"

// LocalPath checks a target that is meant to stay on this site. Every
// server-issued redirect to a local target goes through it, so there is one
// rule (localpath.Is) and not one per handler.
//
// It returns the checked string, so a caller cannot redirect to the unchecked
// one by mistake: use the first result, and only when the second is true.
func LocalPath(target string) (string, bool) {
	if localpath.Is(target) {
		return target, true
	}
	return "", false
}
