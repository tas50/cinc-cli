package cmd

import (
	"encoding/json"
	"fmt"
	"sort"
	"text/tabwriter"
	"time"

	"github.com/spf13/cobra"

	"github.com/cinc-project/cinc-cli/cli/printer"
)

// nodeStatus is one node's check-in summary, as reported by `node status`.
type nodeStatus struct {
	Name            string  `json:"name"`
	CheckinAgo      string  `json:"checkin_ago"`
	Platform        string  `json:"platform,omitempty"`
	PlatformVersion string  `json:"platform_version,omitempty"`
	FQDN            string  `json:"fqdn,omitempty"`
	IPAddress       string  `json:"ipaddress,omitempty"`
	OhaiTime        float64 `json:"ohai_time,omitempty"`
}

// nodeStatusClock is overridable in tests so the relative check-in time is
// deterministic.
var nodeStatusClock = time.Now

// newNodeStatusCmd builds `cinc node status [query]`, which reports every node
// (or those matching the search query) with how long ago it last checked in,
// mirroring knife's `status`. Check-in and host facts are read from each node's
// automatic attributes via a single search.
func newNodeStatusCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "status [query]",
		Short: "Show nodes and when they last checked in",
		Example: `Show every node with how long ago it last checked in.
cinc node status

Limit the report to nodes matching a search query.
cinc node status 'role:web'`,
		Args: cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			format, err := resolveFormat(cmd)
			if err != nil {
				return err
			}
			c, err := resolveClient(cmd)
			if err != nil {
				return err
			}
			query := "*:*"
			if len(args) == 1 && args[0] != "" {
				query = args[0]
			}
			rows, err := c.Search.SearchAll(cmd.Context(), "node", query)
			if err != nil {
				return err
			}
			now := nodeStatusClock()
			statuses := make([]nodeStatus, 0, len(rows))
			for _, row := range rows {
				statuses = append(statuses, nodeStatusFromRow(row, now))
			}
			// Most recently checked-in nodes first; nodes that never checked in
			// (ohai_time 0) sort to the end.
			sort.SliceStable(statuses, func(i, j int) bool {
				return statuses[i].OhaiTime > statuses[j].OhaiTime
			})
			if format == printer.FormatJSON {
				return printer.New(cmd.OutOrStdout(), format).Value(statuses)
			}
			return writeNodeStatusTable(cmd, statuses)
		},
	}
}

// nodeStatusFromRow extracts a node's status from one search result row.
func nodeStatusFromRow(row json.RawMessage, now time.Time) nodeStatus {
	var data any
	_ = json.Unmarshal(row, &data)
	get := func(path ...string) string {
		if v, ok := lookupAttribute(data, path); ok {
			return attributeString(v)
		}
		return ""
	}
	var ohai float64
	if v, ok := lookupAttribute(data, []string{"automatic", "ohai_time"}); ok {
		if f, ok := v.(float64); ok {
			ohai = f
		}
	}
	return nodeStatus{
		Name:            get("name"),
		CheckinAgo:      relativeCheckin(ohai, now),
		Platform:        get("automatic", "platform"),
		PlatformVersion: get("automatic", "platform_version"),
		FQDN:            get("automatic", "fqdn"),
		IPAddress:       get("automatic", "ipaddress"),
		OhaiTime:        ohai,
	}
}

// relativeCheckin renders how long ago a node checked in, given its ohai_time
// (a Unix timestamp in seconds). It returns "never" when there is no check-in.
func relativeCheckin(ohaiUnix float64, now time.Time) string {
	if ohaiUnix == 0 {
		return "never"
	}
	d := now.Sub(time.Unix(int64(ohaiUnix), 0))
	if d < 0 {
		d = 0
	}
	switch {
	case d < time.Minute:
		return fmt.Sprintf("%ds ago", int(d.Seconds()))
	case d < time.Hour:
		return fmt.Sprintf("%dm ago", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%dh ago", int(d.Hours()))
	default:
		return fmt.Sprintf("%dd ago", int(d.Hours()/24))
	}
}

func writeNodeStatusTable(cmd *cobra.Command, statuses []nodeStatus) error {
	tw := tabwriter.NewWriter(cmd.OutOrStdout(), 0, 2, 2, ' ', 0)
	for _, s := range statuses {
		platform := s.Platform
		if s.PlatformVersion != "" {
			platform = platform + " " + s.PlatformVersion
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\n", s.Name, s.CheckinAgo, platform, s.FQDN, s.IPAddress)
	}
	return tw.Flush()
}
