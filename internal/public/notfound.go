package public

import (
	"net/http"
	"strings"

	"github.com/holzcloud/holzcloud-cms/internal/plugin"
)

// maxLoggedPath bounds what is handed on. A scanner will happily ask for a
// kilobyte-long path, and there is no reason to pass it along.
const maxLoggedPath = 200

// scannerNoise are the paths that are obviously not broken links to this site.
//
// Without this filter the list is mostly WordPress and .env probes, and the one
// real broken link an operator needs to see is buried. The filter sits in the
// core rather than in the plugin because it decides whether an event is worth
// waking a module for at all.
var scannerNoise = []string{
	".php", ".asp", ".aspx", ".jsp", ".cgi",
	"/wp-", "/wordpress", "/.env", "/.git", "/vendor/", "/xmlrpc",
	"/phpmyadmin", "/administrator", "/.well-known/traffic-advice",
}

// worthLogging filters out what is not a broken link to this site.
func worthLogging(path string) bool {
	if path == "" || path == "/favicon.ico" || path == "/robots.txt" {
		return false
	}
	lower := strings.ToLower(path)
	for _, noise := range scannerNoise {
		if strings.Contains(lower, noise) {
			return false
		}
	}
	return true
}

// logMiss is the hook the 404 path calls.
//
// The core no longer keeps the list itself. It announces the miss and is done;
// whether anything writes it down is the operator's decision, made by
// installing the plugin or not. That is the whole point of the split: a site
// that does not want a record of what its visitors mistyped simply has no
// module listening.
func (h *Handler) logMiss(r *http.Request, websiteID int64) {
	if h.plugins == nil || !worthLogging(r.URL.Path) {
		return
	}
	h.plugins.Emit(plugin.EventNotFound, websiteID, missEvent(r))
}

// missEvent is what a listening plugin is told about one miss.
//
// Trimmed here rather than in the plugin: the payload crosses into a sandbox
// with a fixed memory budget, and a scanner asking for a kilobyte-long path
// should not be able to decide how much of it gets copied there.
func missEvent(r *http.Request) map[string]string {
	return map[string]string{
		"path":    clip(r.URL.Path, maxLoggedPath),
		"referer": clip(r.Referer(), maxLoggedPath),
	}
}

func clip(s string, max int) string {
	if len(s) > max {
		return s[:max]
	}
	return s
}
