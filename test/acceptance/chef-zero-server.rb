# chef-zero-server.rb — boots an in-memory Chef/Cinc Server for the cinc
# acceptance tests.
#
# Usage: ruby chef-zero-server.rb PORT [ORG]
#
# Starts chef-zero in multi-org mode so that requests use the real
# /organizations/<org>/... routes, creates ORG, and seeds it with a
# small fixed set of nodes, roles, environments, clients, data bags,
# global users, a policy, and a policy group. Groups are the default
# per-org set (admins, billing-admins, clients, users) materialized by
# chef-zero. Runs in the foreground until it receives SIGINT or SIGTERM.
# Prints "READY" once the server is serving requests.

require "chef_zero/server"

port = Integer(ARGV.fetch(0))
org  = ARGV.fetch(1, "acme")

server = ChefZero::Server.new(port: port, host: "127.0.0.1", single_org: false)
server.start_background

# In multi-org mode an org is just a directory; creating it lets the
# DefaultFacade materialize the rest of the org structure on demand.
server.data_store.create_dir(["organizations"], org)

# Fixed seed used by the acceptance tests. Each test starts its own
# chef-zero, so any creates/deletes a test performs are isolated.
server.load_data({
  "nodes"        => { "web01" => {}, "web02" => {}, "db01" => {} },
  "roles"        => { "web" => {}, "db" => {}, "base" => {} },
  "environments" => { "prod" => {}, "staging" => {} },
  "clients"      => { "worker-01" => {}, "worker-02" => {} },
  "data"         => {
    "users" => {
      "alice" => { "id" => "alice", "role" => "admin" },
      "bob"   => { "id" => "bob",   "role" => "viewer" },
    },
    "apps"  => {},
  },
  # Global users (top-level /users, not org-scoped) so `user list` and
  # `user show` have something to fetch alongside the default "pivotal"
  # superuser. These are distinct from the "users" data bag above.
  "users" => {
    "anna" => {
      "username"     => "anna",
      "display_name" => "Anna Admin",
      "email"        => "anna@example.test",
      "first_name"   => "Anna",
      "last_name"    => "Admin",
    },
    "ben" => {
      "username"     => "ben",
      "display_name" => "Ben Viewer",
      "email"        => "ben@example.test",
      "first_name"   => "Ben",
      "last_name"    => "Viewer",
    },
  },
  # One policy with a single revision, pinned into the "prod" group, so
  # `policy show` and `policy-group show` have something to fetch. The
  # revision id is arbitrary but must match between the two.
  "policies" => {
    "appserver" => {
      "1.0.0" => {
        "name"           => "appserver",
        "revision_id"    => "1.0.0",
        "run_list"       => ["recipe[appserver::default]"],
        "cookbook_locks" => {},
      },
    },
  },
  "policy_groups" => {
    "prod" => {
      "policies" => {
        "appserver" => { "revision_id" => "1.0.0" },
      },
    },
  },
}, org)

%w[INT TERM].each { |sig| trap(sig) { server.stop; exit } }

puts "READY"
$stdout.flush
sleep
