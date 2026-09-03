package web

import (
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
func WriteAsset(w http.ResponseWriter, assetPath string, content []byte) {
	ct, ok := assetContentTypes[strings.ToLower(path.Ext(assetPath))]
	if !ok {
		ct = "application/octet-stream"
	}
	w.Header().Set("Content-Type", ct)
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.Write(content)
}
