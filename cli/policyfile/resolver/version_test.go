package resolver

import "testing"

func TestConstraintSatisfies(t *testing.T) {
	cases := []struct {
		constraint string
		version    string
		want       bool
	}{
		{">= 0.0.0", "1.2.3", true},
		{">= 1.0.0", "0.9.0", false},
		{"= 1.2.3", "1.2.3", true},
		{"= 1.2.3", "1.2.4", false},
		{"~> 1.0", "1.4.0", true},
		{"~> 1.0", "2.0.0", false},
		{"~> 2.1.3", "2.1.9", true},
		{"~> 2.1.3", "2.2.0", false},
		{"~> 2.1.3", "2.1.2", false},
		{"> 1.0.0", "1.0.1", true},
		{"> 1.0.0", "1.0.0", false},
		{"< 2.0.0", "1.9.9", true},
		{"<= 2.0.0", "2.0.0", true},
		{"!= 1.0.0", "1.0.1", true},
		{"!= 1.0.0", "1.0.0", false},
		// Greedy-match guard: a pre-release never satisfies a non-pre-release
		// lower bound.
		{">= 1.0.0", "2.0.0-alpha", false},
		{"< 2.0.0", "2.0.0-alpha", true},
	}
	for _, tc := range cases {
		c, err := ParseConstraint(tc.constraint)
		if err != nil {
			t.Fatalf("ParseConstraint(%q): %v", tc.constraint, err)
		}
		v, err := ParseVersion(tc.version)
		if err != nil {
			t.Fatalf("ParseVersion(%q): %v", tc.version, err)
		}
		if got := c.Satisfies(v); got != tc.want {
			t.Errorf("%q satisfies %q = %v, want %v", tc.constraint, tc.version, got, tc.want)
		}
	}
}

func TestNormalizeConstraint(t *testing.T) {
	cases := []struct{ in, want string }{
		{">= 1.0", ">= 1.0.0"},
		{">= 1", ">= 1.0.0"},
		{"~> 2.0", "~> 2.0"},
		{"~> 2.1.3", "~> 2.1.3"},
		{"1.2.3", "= 1.2.3"},
		{">= 0.0.0", ">= 0.0.0"},
	}
	for _, tc := range cases {
		c, err := ParseConstraint(tc.in)
		if err != nil {
			t.Fatalf("ParseConstraint(%q): %v", tc.in, err)
		}
		if got := normalizeConstraint(c); got != tc.want {
			t.Errorf("normalizeConstraint(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestVersionCompare(t *testing.T) {
	cases := []struct {
		a, b string
		want int
	}{
		{"1.0.0", "1.0.0", 0},
		{"1.0.0", "2.0.0", -1},
		{"2.0.0", "1.9.9", 1},
		{"1.2.0", "1.10.0", -1},
		{"1.0.0-alpha", "1.0.0", -1},
		{"1.0.0", "1.0.0-alpha", 1},
		{"1.0.0-alpha", "1.0.0-beta", -1},
	}
	for _, tc := range cases {
		a, _ := ParseVersion(tc.a)
		b, _ := ParseVersion(tc.b)
		if got := a.Compare(b); got != tc.want {
			t.Errorf("Compare(%q, %q) = %d, want %d", tc.a, tc.b, got, tc.want)
		}
	}
}
