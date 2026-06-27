## cinc policy install

Evaluate a Policyfile.rb and write the evaluated lock

### Synopsis

Evaluate a Policyfile.rb and write the evaluated lock.

cinc runs your Policyfile through an embedded CRuby engine (CRuby
compiled to WebAssembly, run with no system Ruby and no CGo), so any
valid Ruby works: loops, conditionals, helper methods, ENV, string
interpolation, and require_relative of sibling files all behave just
as they do with chef. The first run downloads a pinned ruby.wasm and
caches it; later runs are offline.

What it resolves: this command performs evaluation only. It captures
your name, run_list, named run lists, attributes, and each cookbook's
declared source, and writes them to Policyfile.lock.json. It does NOT
yet solve cookbook versions, fetch cookbooks, or compute cookbook
identifiers — so the lock is not a fully-resolved, push-ready lock.
Those resolution steps are a separate, larger feature.

```
cinc policy install [Policyfile.rb] [flags]
```

### Examples

Evaluate ./Policyfile.rb and write ./Policyfile.lock.json.

```bash
cinc policy install
```

Evaluate a specific Policyfile and print the evaluation as JSON.

```bash
cinc policy install path/to/Policyfile.rb --format json
```

### Options

```
  -h, --help            help for install
      --output string   write the lock to this path instead of ./Policyfile.lock.json
```

### Options inherited from parent commands

```
      --config string    path to the Cinc credentials file (default ~/.cinc/credentials)
      --format string    output format: human or json (default "human")
      --profile string   credentials profile to use (default: $CINC_PROFILE, then $CHEF_PROFILE, then "default")
```

### SEE ALSO

* [cinc policy](cinc_policy.md)	 - Manage Policyfile policies on the Cinc Server

