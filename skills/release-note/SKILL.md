---
name: release-note
description: >
  Write the Spectre PS release note for a version from the previous git tag.
  Scan the commits, merged pull requests, plan rows, and markdown changes with
  one agent per area, consolidate them with measured numbers into
  plans/v<ver>/PR/release-v<ver>.md in the shape of the previous note, and
  leave it uncommitted. Use when the user says "release note", "release notes",
  "create the release note for 0.0.5", "draft the release", or runs
  /release-note.
---

# Release note

Write the note only. Do not commit, tag, or push unless the user asks. The
note is the deliverable; the version bump is a separate step.

The previous tag is the newest `v*` tag below the version named:
`git tag --list`. The new version is the one the user named, for example
`0.0.5`.

## 1. Ground the range

```sh
git pull --ff-only
git rev-parse v<old> HEAD
git diff --shortstat v<old>..HEAD
git diff --name-status v<old>..HEAD
git log --merges --oneline v<old>..HEAD
git log --oneline v<old>..HEAD | wc -l
```

Read `plans/v<ver>/00-program.md`, every phase file under `plans/v<ver>/`,
`plans/v<ver>/v<ver>-closure.md`, and the deferred rows in
`plans/v0.0.1/10-deferred.md`. Read the previous note for the shape:
`plans/v0.0.4/PR/release-v0.0.4.md`.

The note covers the code and the markdown since the previous tag. A feature
that exists only in a plan does not ship until the tree proves it.

## 2. Fan out scan agents

One agent per workstream, every one against `v<old>..HEAD`. Fourteen is the
worked example for v0.0.5:

1. Phase 1: graphics device benchmarks.
2. Phase 2: encoder benchmarks.
3. Phase 3: font and metadata benchmarks.
4. Phase 4: reading and writing benchmarks.
5. Phase 5: baseline, budget, and profiles.
6. Phase 6: PDF version compatibility.
7. Phase 7: PDF/A profile corpus and the veraPDF checks.
8. Phase 8 first half: manifest schema, committed corpus, live tier.
9. Phase 8 second half: fonts, PostScript, the gate, fuzzing.
10. Each recovery pull request: the batch2 xref, trailer, and stream fixes.
11. Page tree recovery and the tail refusals.
12. Validation corpus pull requests, the CI workflow, and the bulk tier.
13. Documentation and plans diff, including stale claims.
14. Ledger tallies, tag shas, diff stats, test and benchmark counts, and a
    sweep for changes no plan or pull request names.

Each agent returns facts, not prose: landed rows, file paths, commit shas,
test names, measured numbers, before and after behavior, documented
deviations, and every open or deferred row with its reason. Ask for the
command or file:line that proves each number. Tell the agents not to write
the release note and not to modify any file.

## 3. Consolidate

The note goes to `plans/v<ver>/PR/release-v<ver>.md`. Match the v0.0.4
shape:

- Opening paragraph: the version, the `spectreps version` string, the commit
  range the note covers, and the tree the tag is cut from.
- The ledger paragraph: the phase file count and what each file covers.
- Where the release sits against the deferred ledger.
- Bullets: License, Module, Library, Ledger, Closure, Commits, Pull
  requests, Diff.
- A Highlights table, one row per phase file.
- Install and build, with the `make` targets, a short CLI tour, and a
  library example when the API changed.
- "What landed in v<ver>", one section per area. Each bullet carries its
  measured number, and the behavior flip or recorded deviation when there is
  one. A documentation claim the code contradicts is named as a correction.
- "What this release does not do", naming each deferred row with its reason.
- Known limitations, including stale documentation found during the scan.

Use bold for behavior flips. Apply `skills/unslop/SKILL.md`. Sentence case
headings, straight quotes, periods and commas. Never invent a feature.

## 4. Verify

- Re-run the header numbers: tag shas, `git diff --shortstat`, commit count.
- Count tests and benchmarks when the note quotes them:
  `go test -list '.*' ./...` and `go test -list '^Benchmark' ./...`. A
  benchmark must run with `-run '^$'` to be listed without running.
- Grep the plan files for `[x]`, `[~]`, and `[ ]` and reconcile every row
  count in the note.
- Check that every file path and test name named in the note exists.
- Leave the note uncommitted and report its path and the open questions.

## 5. The version bump is a separate change

When the user asks for the release itself, these move together:

| File | Value |
|------|-------|
| `spectreps/instance.go` | `Version()` returns `<ver>` |
| `internal/ps/op_info.go` | `interpreterVersion` mirrors `<ver>` |
| `spectreps/api_test.go` | the version strings become `<ver>` |

`grep -rn '"<old>"' --include="*.go" .` finds every pin. Then `make lint`
and `make test`. No tag and no push until the user asks.

## 6. The pull request body

Only when the user names it: `plans/PR/pr-release-v<ver>.md` from
`skills/PR/PR_TEMPLATE.md`, citing the note, the version bump, and the
documentation alignment, in the shape of `plans/PR/pr-release-v0.0.4.md`.
