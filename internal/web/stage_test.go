package web

import "testing"

// The label follows the version number: 0.0.x is alpha, 0.1 and later below
// 1.0 is beta, 1.0 on is stable. Anything that is not a version is alpha.
func TestStageOf(t *testing.T) {
	for v, want := range map[string]string{
		"dev": StageAlpha, "": StageAlpha, "a1b2c3d": StageAlpha, "0": StageAlpha,
		"v0.0.1": StageAlpha, "v0.0.25": StageAlpha, "0.0": StageAlpha,
		"v0.1.0": StageBeta, "v0.1.1": StageBeta, "0.3": StageBeta, "v0.9.2-4-gabc1234": StageBeta,
		"v1.0.0-beta.1": StageBeta, "v0.1.1-alpha.2": StageAlpha,
		"v1.0": "", "v1.0.0": "", "2.9.1": "", "v10.2": "",
	} {
		if got := StageOf(v); got != want {
			t.Errorf("StageOf(%q) = %q, want %q", v, got, want)
		}
	}
}
