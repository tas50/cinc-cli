package explore

import "testing"

func TestValidateBrowserURL(t *testing.T) {
	cases := []struct {
		name    string
		url     string
		wantErr bool
	}{
		{"http ok", "http://supermarket.cinc.sh/cookbooks/nginx", false},
		{"https ok", "https://supermarket.cinc.sh/cookbooks/nginx", false},
		{"empty rejected", "", true},
		{"file scheme rejected", "file:///etc/passwd", true},
		{"javascript scheme rejected", "javascript:alert(1)", true},
		{"vscode scheme rejected", "vscode://foo/bar", true},
		{"empty host rejected", "http://", true},
		{"single-dash flag rejected", "-x", true},
		{"double-dash flag rejected", "--foo", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			err := validateBrowserURL(tc.url)
			if tc.wantErr && err == nil {
				t.Fatalf("validateBrowserURL(%q) = nil, want error", tc.url)
			}
			if !tc.wantErr && err != nil {
				t.Fatalf("validateBrowserURL(%q) = %v, want nil", tc.url, err)
			}
		})
	}
}
