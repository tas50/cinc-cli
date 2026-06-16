# 0002 — Expose node summary attributes (platform/version) in the node list

- **Status:** Proposed — partly an ergonomics ask; see "Honest caveat"
- **Affects:** `cinc-api` (`NodesService`), Cinc/Chef Server `/nodes`
- **CLI surface that wants it:** the explorer node list (`cinc explore` → Nodes)

## Summary

`GET /nodes` (and the `NodesService.List` wrapper) returns only a
`name → URL` map — no attributes. So anything that wants a node's
platform, version, last check-in, FQDN, etc. has to fetch the **full**
node object, one `Nodes.Get` per node. That's fine for a single detail
pane, but it makes a useful node *list* (platform/version columns like
`cinc node status` shows) cost one extra round trip per row. We'd like to
read those few summary attributes for many nodes without an N+1 of full
node fetches.

## Where it bites

The explorer node kind lists names only, then fetches the full node on
selection:

```go
// cli/explore/kind_node.go
listFn: func(ctx context.Context, c *cinc.Client) (map[string]string, error) {
	index, _, err := c.Nodes.List(ctx) // <-- names + URLs only
	return index, err
},
getFn: func(ctx context.Context, c *cinc.Client, name string) (*cinc.Node, error) {
	n, _, err := c.Nodes.Get(ctx, name) // <-- full node, just to read platform/version
	return n, err
},
```

The platform/version we render come out of the **full** node's automatic
(ohai) attributes:

```go
// cli/explore/summary.go
func nodeTitle(n *cinc.Node) string {
	platform := n.Automatic.GetString("platform")
	// ...platform_version, etc.
}
```

Because the list has only names, the explorer shows a bare `NAME` column.
To add platform/version columns the way `cinc node status` displays them,
each visible row would need its own `Nodes.Get` — the extra call this entry
is about.

## Why it matters

1. **N+1 to enrich a list.** Showing platform/version (or environment, last
   scan, FQDN) for a screen of nodes means one full-node fetch per node.
2. **Full-object fetches are heavy.** A node's automatic attributes are large;
   pulling the entire document just to read two strings (`platform`,
   `platform_version`) wastes bandwidth and server work.
3. **The list and the detail want different shapes.** The detail pane legitimately
   wants the whole node; the list wants a handful of summary fields for many
   nodes. One endpoint shape can't serve both well today.

## Honest caveat

The Cinc/Chef Server already has a mechanism that returns chosen attributes for
many nodes in a single request: **partial search**. `cinc-api` exposes it via
`Search.Query(ctx, "node", query, WithPartial(keys))`, and `cinc node status`
already uses search to read `automatic.platform` / `automatic.platform_version`
for every node at once (`apps/cinc/cmd/node_status.go`). So the *data* is
reachable in one call today — this is less a missing capability than a missing
**convenient, list-shaped** way to get it without hand-rolling a search query
and digging raw JSON.

## Proposed improvements

In rough order of preference:

1. **A typed summary-list helper in `cinc-api`.** Add something like
   `NodesService.ListSummary(ctx, attrs ...string) ([]NodeSummary, *Response, error)`,
   implemented on top of partial search, returning a small typed struct
   (`Name`, `Platform`, `PlatformVersion`, `Environment`, `LastCheckin`, `FQDN`,
   `IPAddress`). This moves the partial-search plumbing and JSON-digging into the
   library so the explorer (and any caller) gets a rich node list in **one** call
   with no per-node `Get`. Smallest change that removes the N+1.

2. **Let `GET /nodes` return summary attributes.** Server-side, allow the node
   list to include a few summary fields (e.g. `GET /nodes?summary=1` populating
   platform/version/environment/check-in), surfaced by `cinc-api` as a richer
   `List` result. Cleaner than search for a plain list, but needs a server
   change.

3. **Document the partial-search pattern as the blessed path.** If neither lands,
   standardize on `Search.Query(..., WithPartial(...))` for list enrichment and
   factor the attribute-digging out of `node status` so the explorer can reuse it.

Option 1 is the pragmatic target: the server already supports the underlying
request, so this is mostly an ergonomics win in `cinc-api`, and it lets the
explorer show platform/version in the node list without any extra round trips.

## Workaround until then

The explorer can adopt the same partial search `node status` already uses:
issue one `Search.Query(ctx, "node", "*:*", WithPartial({"name": [...], "platform":
[...], "platform_version": [...]}))` to populate the whole list, instead of
`Nodes.List` + per-node `Nodes.Get`. That avoids the N+1 today; this entry asks
for a typed helper so every caller doesn't re-implement that query and parsing.
