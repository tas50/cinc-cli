package explore

import (
	"testing"
	"time"
)

func TestFormatDownloads(t *testing.T) {
	cases := map[int]string{
		0:          "0",
		7:          "7",
		1234:       "1,234",
		142330221:  "142,330,221",
		1000000000: "1,000,000,000",
		-42:        "-42",
		-1234567:   "-1,234,567",
	}
	for in, want := range cases {
		if got := formatDownloads(in); got != want {
			t.Errorf("formatDownloads(%d) = %q, want %q", in, got, want)
		}
	}
}

func TestFormatRelativeTime(t *testing.T) {
	now := time.Date(2026, 5, 22, 12, 0, 0, 0, time.UTC)
	cases := []struct {
		when time.Time
		want string
	}{
		{now.Add(-30 * time.Second), "just now"},
		{now.Add(-2 * time.Minute), "2 min ago"},
		{now.Add(-3 * time.Hour), "3 hr ago"},
		{now.AddDate(0, 0, -1), "yesterday"},
		{now.AddDate(0, 0, -3), "3 days ago"},
		{now.AddDate(0, 0, -21), "3 weeks ago"},
		{now.AddDate(0, -2, 0), "2 months ago"},
		{now.AddDate(-2, 0, 0), "2024-05-22"},
	}
	for _, c := range cases {
		if got := formatRelativeTime(c.when, now); got != c.want {
			t.Errorf("formatRelativeTime(%v) = %q, want %q", c.when, got, c.want)
		}
	}
	if got := formatRelativeTime(time.Time{}, now); got != "—" {
		t.Errorf("zero time = %q, want —", got)
	}
}
