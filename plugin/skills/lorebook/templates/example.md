---
name: example
description: Worked example of a quest template — copy it, then replace every line with something true of your project.
keywords: []
---

<!--
This is the one template fellowship ships. It exists to show the shape and,
more importantly, the specificity bar: every bullet names a file, a command, a
tool, or a rule you could be wrong about. It has no keywords, so it never
auto-suggests; assign it deliberately with `template: example`, or copy it to
`.claude/fellowship-templates/<your-name>.md` and rewrite it.

The guidance below is written for a fictional service that exposes an HTTP API
backed by a SQL database. None of it is true of your project. `/scribe` will
interview you and write one that is.
-->

## Research Guidance

- Read `internal/api/routes.go` end to end before anything else — it is the
  only registry of routes, and a handler that is not listed there is dead.
- For any endpoint that touches money, read `internal/ledger/README.md`. Every
  write to the ledger must be idempotent; the retry semantics are documented
  there and nowhere else.
- Check `db/migrations/` for a migration touching the tables you plan to read.
  A migration merged but not yet deployed means the column exists in `main` and
  not in staging.
- Ask the bulletin whether a sibling quest is already changing the handler you
  landed on: two quests in `routes.go` is the conflict we hit most often.

## Plan Guidance

- Name the migration file in the plan if the change needs one. Migrations are
  never edited after merge, so a plan that says "adjust the schema" is not a
  plan.
- Every new endpoint needs three things planned together: the handler, its row
  in `routes.go`, and its entry in `openapi.yaml`. A PR missing the spec entry
  gets sent back.
- Say which test file each change lands in. Handler tests live beside the
  handler; anything crossing a service boundary goes in `test/integration/`,
  which needs a running database.

## Implement Guidance

- Generate the spec, do not hand-write it: `make openapi` regenerates
  `openapi.yaml` from the route annotations, and a hand-edit is overwritten by
  the next run.
- New tables get a migration through `make migration name=<slug>`, never a bare
  file in `db/migrations/` — the generator writes the timestamp prefix the
  runner sorts on.
- Return `apierr.New(...)`, never a bare `error`, from anything under
  `internal/api/`. The middleware maps `apierr` to a status code and logs the
  rest as a 500.
- Run `make test-unit` as you go; `make test-integration` needs Docker and is
  slow enough that batching it to the end of the phase is fine.

## Review Guidance

- Confirm `openapi.yaml` is regenerated and committed. This is the single most
  common review comment on this repo.
- Confirm every new endpoint has an authorization check. The middleware does
  not apply one by default — an unauthenticated route looks identical to one
  someone forgot.
- Run `make test-integration` before opening the PR, not just the unit tests.
- If the change touches the ledger, say so in the PR body and tag
  `@payments-reviewers`.
