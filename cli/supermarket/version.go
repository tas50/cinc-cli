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
		return strings.Compare(apre, bpre)
	}
}

func splitPrerelease(v string) (base, pre string) {
	if i := strings.IndexAny(v, "-+"); i >= 0 {
		return v[:i], v[i+1:]
	}
	return v, ""
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
