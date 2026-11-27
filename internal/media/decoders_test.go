package media

import (
	"os/exec"
	"strings"
	"testing"
)

// The program links a decoder for every picture kind an upload is allowed to
// be (W-1 of the phase 38 security audit).
//
// image.DecodeConfig knows only the formats something imported. When the GIF
// frame walk replaced gif.DecodeAll, the last production import of image/gif
// went with it, and every GIF then failed DecodeConfig: a still GIF of 3000
// pixels was stored as it was instead of being offered for scaling. The tests
// did not see it, because several test files import image/gif themselves and
// a test binary links what its tests import. So this asks the build of the
// program itself, which no test file can change.
func TestTheProgramLinksADecoderForEveryPictureKind(t *testing.T) {
	goTool, err := exec.LookPath("go")
	if err != nil {
		t.Fatalf("the go command is not on PATH (go test puts it there): %v", err)
	}
	out, err := exec.Command(goTool, "list", "-deps", "-f", "{{.ImportPath}}",
		"github.com/holzcloud/holzcloud-cms/cmd/holzcloud").CombinedOutput()
	if err != nil {
		t.Fatalf("go list: %v\n%s", err, out)
	}
	linked := map[string]bool{}
	for _, p := range strings.Fields(string(out)) {
		linked[p] = true
	}
	for kind, pkg := range map[string]string{
		"image/jpeg": "image/jpeg",
		"image/png":  "image/png",
		"image/gif":  "image/gif",
		"image/webp": "golang.org/x/image/webp",
	} {
		if !allowedMIME[kind] {
			t.Errorf("%s is no longer an allowed upload: update this test", kind)
		}
		if !linked[pkg] {
			t.Errorf("the program does not link %s: image.DecodeConfig fails for every %s", pkg, kind)
		}
	}
}
