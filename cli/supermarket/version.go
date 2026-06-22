package supermarket

import (
	"strconv"
	"strings"
)

// compareSemver compares two version strings using a relaxed semver
// ordering — numeric segment by numeric segment, with any non-numeric
// pre-release suffix ranking lower than a clean release of the same
// base. Returns -1, 0, or 1 in the usual sense.
//
// Supermarket versions are overwhelmingly "X", "X.Y", or "X.Y.Z", with
// the occasional "X.Y.Z-beta1" or similar. This helper only needs to
// pick a winner among those forms, so we don't pull in a full semver
// library.
func compareSemver(a, b string) int {
	if a == b {
		return 0
	}
	if a == "" {
		return -1
	}
	if b == "" {
		return 1
	}
	abase, apre := splitPrerelease(a)
	bbase, bpre := splitPrerelease(b)
	if c := compareNumericSegments(abase, bbase); c != 0 {
		return c
	}
	switch {
	case apre == "" && bpre == "":
		return 0
	case apre == "":
		return 1
	case bpre == "":
		return -1
	default:
		return comparePrerelease(apre, bpre)
	}
}

// splitPrerelease separates a version into its numeric base and pre-release
// suffix. Build metadata (everything after a '+') is discarded entirely, since
// semver §10 says it must be ignored for precedence.
func splitPrerelease(v string) (base, pre string) {
	v, _, _ = strings.Cut(v, "+")
	base, pre, _ = strings.Cut(v, "-")
	return base, pre
}

// comparePrerelease compares two dot-separated pre-release strings per semver
// §11: identifiers are compared left to right; all-numeric identifiers compare
// numerically and rank below alphanumeric ones, and a larger set of fields wins
// when all preceding identifiers are equal.
func comparePrerelease(a, b string) int {
	ai := strings.Split(a, ".")
	bi := strings.Split(b, ".")
	for i := 0; i < len(ai) && i < len(bi); i++ {
		if c := comparePrereleaseIdent(ai[i], bi[i]); c != 0 {
			return c
		}
	}
	switch {
	case len(ai) < len(bi):
		return -1
	case len(ai) > len(bi):
		return 1
	default:
		return 0
	}
}

func comparePrereleaseIdent(a, b string) int {
	an, aerr := strconv.Atoi(a)
	bn, berr := strconv.Atoi(b)
	switch {
	case aerr == nil && berr == nil:
		switch {
		case an < bn:
			return -1
		case an > bn:
			return 1
		default:
			return 0
		}
	case aerr == nil: // numeric identifiers rank below alphanumeric ones
		return -1
	case berr == nil:
		return 1
	default:
		return strings.Compare(a, b)
	}
}

func compareNumericSegments(a, b string) int {
	ap := strings.Split(a, ".")
	bp := strings.Split(b, ".")
	n := max(len(ap), len(bp))
	for i := range n {
		ai := segmentInt(ap, i)
		bi := segmentInt(bp, i)
		if ai != bi {
			if ai < bi {
				return -1
			}
			return 1
		}
	}
	return 0
}

func segmentInt(parts []string, i int) int {
	if i >= len(parts) {
		return 0
	}
	n, err := strconv.Atoi(parts[i])
	if err != nil {
		return 0
	}
	return n
}
