# Acceptance seed data

This directory is a [chef-repo][] that the acceptance harness hands to
`cinc-zero --repo` to preload a fixed set of objects into the `acme` org before
each test. It replaces the programmatic seed the old Ruby `chef-zero` helper
built with `server.load_data`.

`cinc-zero` loads these on startup, mirroring `knife upload`:

- `nodes/` — `web01`, `web02`, `db01`
- `roles/` — `web`, `db`, `base`
- `environments/` — `prod`, `staging` (plus the `_default` env cinc-zero always creates)
- `clients/` — `worker-01`, `worker-02`
- `data_bags/users/` — items `alice`, `bob`; `data_bags/apps/` — an empty bag (kept by `.gitkeep`)
- `policies/appserver-1.0.0.json` — a Policyfile lock loaded as revision `1.0.0`
- `policy_groups/prod.json` — pins `appserver` `1.0.0` into the `prod` group

Two things the chef-repo format can't express are seeded separately by the
harness (see `helpers_test.go`), against the running `--no-auth` server:

- the **global users** `anna` and `ben` (`/users` is global, not org-scoped)
- the **`devs`** authz group (the loader has no `groups/` directory)

cinc-zero auto-creates the default groups (`admins`, `billing-admins`,
`clients`, `users`), so those need no seeding.

[chef-repo]: https://docs.chef.io/chef_repo/
