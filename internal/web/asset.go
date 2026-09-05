package web

import (
	"crypto/sha256"
	"encoding/base64"
	"net/http"
	"path"
	"strings"
)

// assetContentTypes maps template asset extensions to their Content-Type.
// Anything not listed is served as an opaque download.
var assetContentTypes = map[string]string{
	".css":   "text/css; charset=utf-8",
	".js":    "application/javascript; charset=utf-8",
	".svg":   "image/svg+xml",
	".png":   "image/png",
	".jpg":   "image/jpeg",
	".jpeg":  "image/jpeg",
	".gif":   "image/gif",
	".webp":  "image/webp",
	".ico":   "image/x-icon",
	".woff":  "font/woff",
	".woff2": "font/woff2",
	".ttf":   "font/ttf",
}

// WriteAsset writes template asset bytes with the Content-Type derived from the
// file extension. Sniffing is disabled so a mislabelled asset cannot be
// reinterpreted as HTML by the browser.
//
// It reports whether anything was written: a request that already had the file
// gets a 304 and no body.
func WriteAsset(w http.ResponseWriter, r *http.Request, assetPath string, content []byte) bool {
	ct, ok := assetContentTypes[strings.ToLower(path.Ext(assetPath))]
	if !ok {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")

	etag := AssetETag(content)
	w.Header().Set("ETag", etag)
	if matchesETag(r.Header.Get("If-None-Match"), etag) {
		w.WriteHeader(http.StatusNotModified)
		return false
	}
	w.Write(content)
	return true
}

// AssetETag is a strong validator over the bytes themselves.
//
// Over the content and not the modification time: the same template ships in
// the binary and may also lie on disk, and a file that was merely re-extracted
// has a new timestamp and the same bytes. A visitor should not download it
// again for that.
func AssetETag(content []byte) string {
	sum := sha256.Sum256(content)
	return `"` + base64.RawURLEncoding.EncodeToString(sum[:16]) + `"`
}

// matchesETag implements the If-None-Match comparison, which is a list and not
// a single value — and "*" means "any representation I might already have".
func matchesETag(header, etag string) bool {
	if header == "" {
		return false
	}
	for _, part := range strings.Split(header, ",") {
		part = strings.TrimSpace(part)
		if part == "*" || part == etag || strings.TrimPrefix(part, "W/") == etag {
			return true
		}
	}
	return false
}
