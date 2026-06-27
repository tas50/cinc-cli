package resolver

import "testing"

// TestComputeIdentifierGoldenValues pins cinc's cookbook identifier computation
// to the exact content/dotted-decimal identifiers real `chef install` produced
// for the testdata cookbooks (captured from the committed goldens). These are
// the bytes a Chef Infra Server addresses a cookbook artifact by, so they must
// match chef exactly.
func TestComputeIdentifierGoldenValues(t *testing.T) {
	cases := []struct {
		dir           string
		content       string
		dottedDecimal string
	}{
		{
			dir:           "testdata/transitive/cookbooks/alpha",
			content:       "3817872381e3cbf5d8bcbc16bced7e5e5438344b",
			dottedDecimal: "15788467879535563.69199674415168749.138943604995147",
		},
		{
			dir:           "testdata/transitive/cookbooks/beta",
			content:       "484874ca84fbec8d44006eb2a1840781c331292f",
			dottedDecimal: "20345864774286316.39762740364091780.8253906954543",
		},
		{
			dir:           "testdata/transitive/cookbooks/gamma",
			content:       "194c08b2a394d35a12d38482a060c2cdf63df9b3",
			dottedDecimal: "7120474658280659.25353447574511712.214189855340979",
		},
		{
			// chefignore must exclude *.bak / *.tmp, matching chef.
			dir:     "testdata/chefignore_cookbook/cookbooks/widget",
			content: "e80db6497ca7c1baab2f722b0372f6bc57e8bbba",
		},
	}

	for _, tc := range cases {
		t.Run(tc.dir, func(t *testing.T) {
			id, err := ComputeIdentifier(tc.dir)
			if err != nil {
				t.Fatal(err)
			}
			if id.Content != tc.content {
				t.Errorf("content identifier = %q, want %q", id.Content, tc.content)
			}
			if tc.dottedDecimal != "" && id.DottedDecimal != tc.dottedDecimal {
				t.Errorf("dotted-decimal identifier = %q, want %q", id.DottedDecimal, tc.dottedDecimal)
			}
		})
	}
}

// TestChefignoreExcludesIgnoredFiles confirms the identifier file set drops
// chefignored paths (so an ignored scratch file cannot change the identifier).
func TestChefignoreExcludesIgnoredFiles(t *testing.T) {
	files, err := cookbookFiles("testdata/chefignore_cookbook/cookbooks/widget")
	if err != nil {
		t.Fatal(err)
	}
	for _, f := range files {
		if f.Path == "recipes/default.rb.bak" || f.Path == "notes.tmp" {
			t.Errorf("chefignored file %q was included in the identifier set", f.Path)
		}
	}
	// The chefignore file itself and the real cookbook files remain.
	want := map[string]bool{"metadata.rb": false, "recipes/default.rb": false, "chefignore": false}
	for _, f := range files {
		if _, ok := want[f.Path]; ok {
			want[f.Path] = true
		}
	}
	for path, found := range want {
		if !found {
			t.Errorf("expected file %q in identifier set, missing", path)
		}
	}
}
