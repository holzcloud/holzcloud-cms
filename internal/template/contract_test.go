package template

import (
	"strings"
	"testing"
	"testing/fstest"
)

// A theme says which contract it was written for in theme.json, and nothing
// else in that file is read. Absent is allowed, any well-formed major.minor is
// reported as it is, and only a file that cannot be read as that shape is an
// error — one naming theme.json, so the author knows which file to open.
func TestDeclaredContract(t *testing.T) {
	cases := []struct {
		name     string
		manifest *string
		version  string
		declared bool
		wantErr  bool
	}{
		{name: "no theme.json", manifest: nil},
		{name: "the current contract", manifest: ptr(`{"contract": "1.0"}`), version: "1.0", declared: true},
		{name: "another version, other keys ignored", manifest: ptr(`{"contract": "2.3", "name": "x"}`), version: "2.3", declared: true},
		{name: "an object without contract", manifest: ptr(`{"name": "x"}`)},
		{name: "major only", manifest: ptr(`{"contract": "1"}`), wantErr: true},
		{name: "three parts", manifest: ptr(`{"contract": "1.0.0"}`), wantErr: true},
		{name: "a prefix", manifest: ptr(`{"contract": "v1.0"}`), wantErr: true},
		{name: "a number, not a string", manifest: ptr(`{"contract": 1.0}`), wantErr: true},
		{name: "an array", manifest: ptr(`[]`), wantErr: true},
		{name: "not JSON", manifest: ptr(`not json`), wantErr: true},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			theme := fstest.MapFS{"layout.html": &fstest.MapFile{Data: []byte(goodLayout)}}
			if tc.manifest != nil {
				theme[ManifestName] = &fstest.MapFile{Data: []byte(*tc.manifest)}
			}

			version, declared, err := DeclaredContract(theme)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("DeclaredContract accepted %s, answering %q, %v", *tc.manifest, version, declared)
				}
				if !strings.Contains(err.Error(), "theme.json") {
					t.Errorf("the error does not name theme.json, so the author does not know which file to open: %v", err)
				}
				return
			}
			if err != nil {
				t.Fatalf("DeclaredContract refused a well-formed manifest: %v", err)
			}
			if version != tc.version || declared != tc.declared {
				t.Errorf("DeclaredContract = %q, %v; want %q, %v", version, declared, tc.version, tc.declared)
			}
		})
	}
}

func ptr(s string) *string { return &s }
