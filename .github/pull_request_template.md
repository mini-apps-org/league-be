## Why

What problem does this solve (maximum 3 sentences)? Link the ticket.

## What changed

- Describe the smallest useful change.
- Call out anything intentionally left unchanged.

## Blast radius

- **Level:** ROOT (`internal/platform/*`, config, middleware, a migration) / TRUNK (one feature package, e.g. `internal/auth`) / LEAF (one handler, one query)
- **What must not change:** error codes already shipped, response shapes the SPA reads, cookie names/attributes, config keys.
- **How far can failure travel:** which routes and which packages import this.
- **Gate / rollback:** plain revert, or is something not revertable — an applied migration, a rotated key, a reassigned error code?

## Manual steps this merge does NOT perform

- [ ] Migration to apply: `psql "$DATABASE_URL" -f db/migrations/…` (give the order)
- [ ] New environment variable to set in the deploy target (and in `.env.example`)
- [ ] Secret to rotate
- [ ] None of the above

## How to test

1. `make run`, then the exact `curl` (method, path, cookies, `X-CSRF-Token` where unsafe).
2. The expected status code, error code and body at each step.

## Useful code paths

- `internal/<feature>/handlers.go`
- `internal/<feature>/services.go`
- `internal/<feature>/repositories.go`
- `db/migrations/…`

## Validation

- [ ] `test -z "$(gofmt -l .)"`
- [ ] `make vet`
- [ ] `make test` passes
- [ ] `make swagger` run and `docs/swagger/*` committed, if a handler or annotation changed
- [ ] New error codes added to the registry in `internal/platform/handler/error.go`, none renumbered
- [ ] New config keys added to all three of `development.yaml` / `staging.yaml` / `production.yaml`
- [ ] Repository changes follow [`.claude/rules/database.md`](../.claude/rules/database.md)

## Proof

Paste the request/response for the happy path and for at least one failure path.
