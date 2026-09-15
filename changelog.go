// Package holzcloud exists for one file.
//
// CHANGELOG.md belongs at the root of a repository: that is where GitHub looks
// for it, where a contributor looks for it, and where CONTRIBUTING.md tells
// somebody to add their entry. go:embed can only reach files inside its own
// package's directory, and a path with ".." in it is rejected outright — so
// embedding it means there has to be a package here.
//
// The alternative was a copy under internal/ kept in step by a -check tool, the
// way the themes' catalogues are. That is the right shape for something
// GENERATED from a source; a changelog is written by hand, by whoever made the
// change, and a second copy of it is a second thing to forget.
//
// internal/tmplspec does it the other way round — TEMPLATE-SPEC.md lives inside
// the package that serves it — and that is right for a document nobody expects
// at the root. This one is expected there.
package holzcloud

import _ "embed"

// Changelog is CHANGELOG.md, verbatim.
//
// Parsed by internal/changelog and shown to the operator behind the version
// number in the sidebar. Verbatim and not pre-parsed, because the parsing
// belongs with the rendering and this package is a door, not a room.
//
//go:embed CHANGELOG.md
var Changelog string
