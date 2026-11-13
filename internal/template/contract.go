package template

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/fs"
	"regexp"
)

// ContractVersion names the contract TEMPLATE-SPEC.md documents: the data a
// template receives, the helpers it may call, the files it may carry and the
// rules it has to keep. It is major.minor and starts at 1.0.
//
// A theme may say which contract it was written for, in theme.json, and
// `holzcloud template check` reports that beside this constant. This package
// states and compares versions and draws no conclusion from a difference: what
// an older or a newer declared version means for rendering is future work
// (REQUIREMENTS.md), and until it is decided nothing here pretends to know.
const ContractVersion = "1.0"

// ManifestName is the file at a theme's root that may declare the contract.
// Only its "contract" key is read; anything else in it is ignored.
const ManifestName = "theme.json"

// contractPattern is major.minor and nothing else: no "v", no patch number.
var contractPattern = regexp.MustCompile(`^[0-9]+\.[0-9]+$`)

// DeclaredContract reads the contract version a theme declares in theme.json.
//
// A theme without theme.json, or with one that has no "contract" key, declares
// nothing, and that is allowed: declared is false and err is nil. A file that
// is not a JSON object, whose "contract" is not a string, or whose string is
// not major.minor is an error naming theme.json — the author has said
// something, and it cannot be read.
func DeclaredContract(theme fs.FS) (version string, declared bool, err error) {
	raw, err := fs.ReadFile(theme, ManifestName)
	if errors.Is(err, fs.ErrNotExist) {
		return "", false, nil
	}
	if err != nil {
		return "", false, fmt.Errorf("%s cannot be read: %w", ManifestName, err)
	}

	// A map of raw values, so that a missing "contract" and one of the wrong
	// type are two different answers: the first declares nothing, the second
	// is a mistake the author has to hear about.
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(raw, &fields); err != nil || fields == nil {
		return "", false, fmt.Errorf(`%s is not a JSON object like {"contract": "%s"}`, ManifestName, ContractVersion)
	}

	value, ok := fields["contract"]
	if !ok {
		return "", false, nil
	}
	if err := json.Unmarshal(value, &version); err != nil {
		return "", false, fmt.Errorf(`%s: "contract" must be a string like "%s"`, ManifestName, ContractVersion)
	}
	if !contractPattern.MatchString(version) {
		return "", false, fmt.Errorf(`%s: "contract" must be major.minor like "%s", not %q`, ManifestName, ContractVersion, version)
	}
	return version, true, nil
}
