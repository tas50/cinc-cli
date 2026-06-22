package supermarket

import "testing"

func TestCompareSemver(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		// Plain numeric ordering.
		{"2.0.0", "1.9.9", 1},
		{"1.0.0", "1.0.0", 0},
		{"1.2.0", "1.10.0", -1}, // numeric, not lexical
		{"1.0", "1.0.0", 0},     // implied-zero patch

		// A clean release outranks any prerelease of the same base.
		{"1.0.0", "1.0.0-beta", 1},
		{"1.0.0-beta", "1.0.0", -1},

		// Dot-separated numeric prerelease identifiers compare numerically
		// (the genuine fix — lexical compare ordered "alpha.10" before "alpha.2").
		{"1.0.0-alpha.10", "1.0.0-alpha.2", 1},
		{"1.0.0-alpha.1.1", "1.0.0-alpha.1", 1},

		// An un-dotted identifier ("beta10") is a single alphanumeric token and,
		// per semver §11, compares lexically — so "beta2" outranks "beta10".
		// This is standard semver behavior, not a regression.
		{"1.0.0-beta10", "1.0.0-beta2", -1},

		// Numeric prerelease identifiers rank below alphanumeric ones (semver §11).
		{"1.0.0-1", "1.0.0-alpha", -1},

		// A larger set of prerelease fields has higher precedence.
		{"1.0.0-alpha.1", "1.0.0-alpha", 1},

		// Build metadata is ignored for precedence (semver §10).
		{"1.0.0+build5", "1.0.0", 0},
		{"1.0.0+build5", "1.0.0+build2", 0},
		{"1.0.0-beta+build", "1.0.0-beta", 0},
	}
	for _, c := range cases {
		if got := compareSemver(c.a, c.b); got != c.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d", c.a, c.b, got, c.want)
		}
		// Comparison must be antisymmetric.
		if got := compareSemver(c.b, c.a); got != -c.want {
			t.Errorf("compareSemver(%q, %q) = %d, want %d (antisymmetry)", c.b, c.a, got, -c.want)
		}
	}
}
