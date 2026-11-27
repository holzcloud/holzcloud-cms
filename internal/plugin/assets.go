package plugin

import "path"

// assetTypes is the one table of files a plugin may ship under assets/.
//
// Assets are served under /plugin-assets on every host, including the ones
// where an administrator's session lives. A script on that address would be a
// script on the origin that holds the session, so only files a browser never
// executes are allowed: stylesheets, images, fonts, JSON and text. Scripts
// (.js, .mjs, .cjs, .wasm) and documents (.html, .xhtml, .xml, .xsl) are not on
// the list, and neither is a file without an extension.
//
// An SVG is an image that can carry script, so it is the one entry that is only
// ever sent as a download.
var assetTypes = map[string]struct {
	contentType string
	download    bool
}{
	".css":   {"text/css; charset=utf-8", false},
	".png":   {"image/png", false},
	".jpg":   {"image/jpeg", false},
	".jpeg":  {"image/jpeg", false},
	".gif":   {"image/gif", false},
	".webp":  {"image/webp", false},
	".avif":  {"image/avif", false},
	".ico":   {"image/x-icon", false},
	".woff2": {"font/woff2", false},
	".woff":  {"font/woff", false},
	".json":  {"application/json", false},
	".txt":   {"text/plain; charset=utf-8", false},
	".svg":   {"image/svg+xml", true},
}

// AssetType says whether a plugin may ship name as an asset and, if so, what it
// is served as and whether it is served as a download.
//
// The same table decides at install (ReadPackage) and at serve
// (HandlePluginAsset), so the two can never disagree: a file an older
// installation still has on disk is judged by today's table when it is asked
// for, not by the one in force when it was written.
func AssetType(name string) (contentType string, download bool, ok bool) {
	ext := []byte(path.Ext(name))
	for i, c := range ext {
		if c >= 'A' && c <= 'Z' {
			ext[i] = c + 'a' - 'A'
		}
	}
	e, found := assetTypes[string(ext)]
	if !found {
		return "", false, false
	}
	return e.contentType, e.download, true
}
