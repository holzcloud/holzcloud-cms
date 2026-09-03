// Package tmplspec carries the template authoring specification.
//
// The document lives here rather than under docs/ so it can be embedded: it has
// to be reachable from the running binary, because the person — or agent —
// writing a template usually has the binary and not the source tree. One copy,
// three ways out: `holzcloud template spec`, the admin UI, and the file itself
// in the repository.
package tmplspec

import _ "embed"

//go:embed TEMPLATE-SPEC.md
var markdown string

// Markdown returns the specification.
func Markdown() string { return markdown }
