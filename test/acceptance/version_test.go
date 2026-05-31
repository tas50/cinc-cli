//go:build acceptance

package acceptance

import (
	"strings"
	"testing"
)

// TestVersionAgainstRealBinary runs the real cinc binary's version
// command. It does not need cinc-zero. The build injects empty
// fallbacks for VERSION/COMMIT/BUILD_DATE when go test compiles the
// binary, so the test asserts on structure rather than exact values.
func TestVersionAgainstRealBinary(t *testing.T) {
	binary := buildCinc(t)

	out := runCinc(t, binary, "version")
	for _, want := range []string{"cinc", "commit", "built"} {
		if !strings.Contains(strings.ToLower(out), want) {
			t.Errorf("version output missing %q\ngot: %s", want, out)
		}
	}
}
