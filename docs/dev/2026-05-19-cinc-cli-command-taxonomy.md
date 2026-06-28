# `cinc` CLI: Command Taxonomy Design

- **Date:** 2026-05-19
- **Status:** Draft (for review)
- **Scope:** Command taxonomy only (verbs, nouns, grouping, UX). Implementation
  architecture (language, plugin model, server API layer, config format) is
  explicitly out of scope for this document.

## Background

The Cinc/Chef command-line experience is currently split across multiple
existing ecosystem tools, which use two distinct and inconsistent grammars:

- Server interaction (nodes, roles, cookbooks, data bags, users, ACLs) uses a
  **noun-verb** grammar.
- Workstation authoring tasks (scaffolding, Policyfiles, exporting) use a
  **verb-noun** grammar.

This split forces users to learn two grammars and remember which tool owns which
task. `cinc` replaces them with a single command and a single, consistent
grammar.

## Design Decisions

The taxonomy was settled through the following decisions:

1. **Noun-verb ordering.** Every command is `cinc <noun> <verb>`. Grouping by the
   resource being acted on maximizes per-object discoverability and matches
   widely-used noun-verb CLIs such as `gh` and `docker`.
2. **Policyfiles and roles/environments are equal footing.** `policy`, `role`,
   and `environment` are all first-class noun groups with no "legacy" labeling.
3. **`create` is the universal "bring into being" verb.** For server objects it
   creates on the server; for local artifacts (`cookbook`, `repo`, `policy`) it
   scaffolds on disk. (This replaces the separate `generate` verb used by
   existing authoring tools.)
4. **Cookbook-internal scaffolding uses `cinc cookbook add <part>`.**
5. **A short, documented set of global utility verbs** is allowed at the top
   level for commands that have no natural noun.
6. **`exec` is dropped**: no "run inside the Cinc/Chef Ruby runtime" escape
   hatch.

## Command Grammar & Conventions

```
cinc <noun> <verb> [target] [--flags]      # object commands
cinc <verb> [args] [--flags]               # global utilities (short, fixed list)
```

- **Strict noun-verb.** One resource = one group = one place to discover every
  action on it.
- **Consistent core verbs.** `list`, `show`, `create`, `edit`, `delete` mean the
  same thing on every noun. Resource-specific verbs (`upload`, `bootstrap`,
  `push`) are additive.
- **`create` is overloaded by design.** `cinc role create` hits the server;
  `cinc cookbook create` scaffolds local files. Same word, same intent ("bring
  this into being"); each noun's help disambiguates.
- **Global utility verbs are a deliberate, documented short list**, not an open
  category.

## Command Reference

### Server-object nouns

```
cinc node          list  show  create  edit  delete  bootstrap  ssh  status
                   run-list <add|remove|set>   policy-set   environment-set
                   tag <add|remove|list>

cinc cookbook      list  show  upload  download  delete  create  metadata  bump
                   add <recipe|resource|template|attribute|file|helpers>

cinc role          list  show  create  edit  delete   run-list <add|remove|set>

cinc environment   list  show  create  edit  delete   compare

cinc databag      list  show  create  edit  delete
                   item <list|show|create|edit|delete>      (encrypted via --secret)

cinc client        list  show  create  delete  reregister
                   key <list|show|create|delete|edit>

cinc user          list  show  create  edit  delete  password
                   key <list|show|create|delete|edit>

cinc group         list  show  create  edit  delete   member <add|remove>
```

### Policy nouns

```
cinc policy        list  show  create  install  update  push  diff  export  delete  clean

cinc policy-group  list  show  diff  delete
```

### Authoring & workstation nouns

```
cinc repo          create  diff                  (diff = local repo vs. server)

cinc supermarket   search  show  install  share  unshare  list

cinc config        list  show  use  edit  path
```

### Server administration

```
cinc server        ssl <check|fetch>
                   acl <show|add|remove|bulk>
```

### Global utility verbs

```
cinc search <index> <query>      Cross-object query against the server
cinc shell-init <shell>          Emit shell env setup for cinc's context
cinc version                     Version info
cinc help                        Help system
```

**Totals: 14 noun groups + 4 global utility verbs.**

## Out of Scope / Open Questions

These are deliberately deferred and not decided by this document:

- **Implementation language** and runtime.
- **Plugin model**: how third-party commands (notably cloud provisioning) extend
  `cinc`.
- **Config file format** for `cinc config` and credential/profile storage.
- **Whether a local in-memory server mode** should return under a future
  `cinc local` or `cinc server` namespace.
- **`exec`/`gem` replacements**: if a future need arises for running tooling in a
  managed environment, it would need a fresh design rather than reviving `exec`.
