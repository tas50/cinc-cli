#!/usr/bin/env bash
#
# generate_goldens.sh regenerates the golden Policyfile.lock.json for every
# fixture in this directory by running REAL `chef install` (chef-cli) against
# it. The committed goldens are the compatibility oracle the resolver tests
# diff against, so they must come from chef, not from cinc.
#
# Each fixture is a directory holding a Policyfile.rb and a cookbooks/ tree. We
# copy the fixture to a scratch dir OUTSIDE any git repository (so chef's
# NullSCM is used and scm_info stays null, keeping the golden portable), run
# chef install there, and copy the resulting lock back into the fixture.
#
# Requirements: chef-cli on PATH (the repo was developed against chef-cli
# 6.1.34). Run from anywhere:
#
#     ./cli/policyfile/resolver/testdata/generate_goldens.sh
#
set -euo pipefail

HERE="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
CHEF="${CHEF_CLI_BIN:-chef-cli}"
export CHEF_LICENSE="${CHEF_LICENSE:-accept-no-persist}"

if ! command -v "$CHEF" >/dev/null 2>&1; then
  echo "error: chef-cli not found on PATH (set CHEF_CLI_BIN to override)" >&2
  exit 1
fi

echo "Using $($CHEF --version 2>/dev/null | head -1)"

for fixture in "$HERE"/*/; do
  name="$(basename "$fixture")"
  [ -f "$fixture/Policyfile.rb" ] || continue

  scratch="$(mktemp -d "${TMPDIR:-/tmp}/cinc-golden-$name.XXXXXX")"
  # Copy the fixture sources (Policyfile + cookbooks) into the non-git scratch.
  cp "$fixture/Policyfile.rb" "$scratch/"
  cp -R "$fixture/cookbooks" "$scratch/cookbooks"

  echo "==> $name"
  if ( cd "$scratch" && "$CHEF" install Policyfile.rb >/dev/null 2>"$scratch/err.log" ); then
    cp "$scratch/Policyfile.lock.json" "$fixture/Policyfile.lock.json"
    echo "    wrote $name/Policyfile.lock.json"
  else
    echo "    chef install FAILED for $name:" >&2
    sed 's/^/      /' "$scratch/err.log" >&2 || true
  fi
  rm -rf "$scratch"
done

echo "Done."
