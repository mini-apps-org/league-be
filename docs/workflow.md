# [WIP] Conventions, and the path from a feature branch to a deployed image

Two halves. §1–§6 are the conventions a change must satisfy before it is worth
committing. §7–§10 are the steps, in order, from branch to running image.

SQL is out of scope here — `internal/*/repositories.go` follows
[`.claude/rules/database.md`](../.claude/rules/database.md) instead.

---

## 1. Where code goes

```
internal/<feature>/          one vertical slice, one package
  handlers.go                HTTP only: read the request, call the service, write the response
  services.go                business rules; `Service` is an interface, `service` the impl
  repositories.go            the only SQL; `Repository` is an interface
  models.go                  domain/db structs, `db:"col"` tags
  model_services.go          the service boundary types (inputs/results)
  model_handlers.go          the JSON wire types, camelCase `json:` tags
internal/platform/           shared machinery, no business rules
  config/ db/ handler/ middleware/ model/ router/
internal/{common,secret}/     leaf utilities
```

Dependencies point inward and never back out:

```
router → <feature> → platform/{config,db,handler,model} → {common,secret}
```

A feature package never imports `router`, and `platform` never imports a feature.
When platform code needs a feature's value, it is **injected**, not imported —
`middleware.NewCSRF(cfg, auth.CSRFCookieName(cfg))` is the pattern.

Split a file before it gets long. `handlers_cookie.go` exists because cookie
attributes are one concern and one concern gets one file.

---

## 2. Errors

- Every error gets wrapped with package and operation: `fmt.Errorf("auth: complete login: %w", err)`.
- A repository returns its **own sentinel** (`ErrNoUser`), never `pgx.ErrNoRows`.
- The **service** maps a sentinel to an API error: `handler.Error(handler.CodeNotFound, err)`.
- The **handler** calls `h.ReturnWithError(w, r, err)` and nothing else. Anything that is
  not an `*APIError` is served as `50001`, so an unclassified failure cannot leak its text.

Error codes live in one registry: `internal/platform/handler/error.go`. The code is
5 digits — HTTP status, then the case within that status. To add one:

1. Add the `const` under its status block, taking the next free number.
2. Add its `errorDetails` entry: a stable `reason` slug for logs, a `message` safe for a browser.
3. Name it in the handler's swagger `@Failure` line.

**Never renumber or reassign a shipped code** — the frontend switches on the string.
Retired codes (`40002`, `40303`) stay retired and are marked as such in the file.

---

## 3. HTTP

- Success body is always `config.SuccessResponse{Data: ...}` via `h.ReturnWithJSON`.
- Request bodies go through `h.DecodeJSON` — it caps the body and rejects unknown fields.
- List endpoints page by keyset. Call `model.CursorParams.Normalise()` in the service
  so an unbounded scan cannot be requested.
- Wire structs are separate from db structs. `model_handlers.go` holds the response shape
  and a `newXResponse(...)` mapper; a `db:"..."` struct is never serialized directly.
- Routes are registered in `internal/platform/router/<feature>.go` as
  `registerXRoute(ctx, api)`, and wired from `NewRouter`. The feature's constructors
  (`NewRepo` → `NewService` → `NewHandler`) are assembled there and nowhere else.
- Mounting under `PrefixAPIV1` gives you CSRF on every unsafe method for free. Auth is
  opt-in per route: `authHandler.AuthenticationWithSession(http.HandlerFunc(h.Me))`.
- Every handler carries swagger annotations — `@Summary`, `@Tags`, `@Success`, one
  `@Failure` per code it can return, `@Security CSRFToken` on unsafe methods, `@Router`.

---

## 4. Config

Per-environment YAML is **compiled into the binary** (`go:embed`), overlaid with
environment variables. So:

- Non-secret, per-environment values → `development.yaml`, `staging.yaml`, `production.yaml`.
  Add the field to all three, or the environment you forgot gets a zero value.
- Secrets and per-machine values → environment only, read in `config.Load()`, and
  documented in `.env.example`.
- Validate in `Load()` and **fail closed**: an empty `domains.allowed_origins` stops the
  process rather than letting `rs/cors` read it as "every origin".
- Editing a YAML file has no effect until the binary is rebuilt. `.air.toml` already
  watches `yaml` for this reason.

---

## 5. Database changes

- Schema: `db/migrations/NNNNN_name.sql`, wrapped in `BEGIN; … COMMIT;`, every object
  `IF NOT EXISTS`. Ids are `TEXT` holding `uuid_generate_v4()`, so every FK into them is
  `TEXT` too.
- Dev data: `db/migrations/seeds/YYYYMMDD_seed.sql`, keyed on natural keys only, so it
  re-reads top to bottom without depending on generated uuids.
- No migration runner is wired into the Makefile — apply with `psql "$DATABASE_URL" -f <file>`
  and say so in the PR description, including the order.

---

## 6. Tests

- `*_test.go` beside the code, stdlib `testing`, sub-tests over table cases.
- No database and no network: `Repository` and `IdentityProvider` are interfaces so the
  service is faked. `*secret.Box` is deliberately *not* an interface — tests exercise the
  real crypto.
- A test earns its place by guarding something the compiler cannot see. The two router
  tests exist because a cookie `Path` that stops covering its route breaks login with no
  compile error and nothing in the logs.
- A comment on the test says what breaks in production when it fails.

---

## 7. Build the feature

```
1. git switch -c feat/<slug>          # off master; fix/<slug> for a bug, refactor/… etc.
2. migration first, if the schema moves (§5)
3. implement inward-out: models → repositories → services → handlers → router
4. make swagger                        # regenerate; docs/swagger/* is committed
5. tests for what the compiler cannot catch (§6)
```

Verify before committing — this is exactly what CI will run:

```
test -z "$(gofmt -l .)"    # CI fails the build on any unformatted file
make vet
make test                  # vet + go generate + go test -failfast
make run                   # air; smoke the endpoint by hand
curl localhost:9200/api/v1/health
```

---

## 8. Commit

Conventional commits, lowercase imperative subject, no trailing period:

```
<type>(<scope>): <subject>
```

`type` ∈ `feat` `fix` `refactor` `docs` `chore`. `scope` is the package or area —
`auth`, `db`, `config`, `cors`, `handler`, `middleware`, `secret`, `health`, `migration`,
`changelog`. Real examples from this history:

```
feat(auth): Google OAuth sign-in with server-side sessions
fix(cors): drive allowed origins from config, and narrow them per environment
refactor(auth): move the session sweep onto Service and share RandomToken
```

Say *what changed and why*, not which files moved. One logical change per commit; if the
working tree has grown several unrelated ones, split them before committing.

---

## 9. Pull request

```
1. git push -u origin feat/<slug>
2. open the PR against master
3. CI must be green: golangci-lint → gofmt → go test -race → go build
4. merge (history keeps the merge commit, so the PR number stays in the log)
```

The PR body lists any manual step the merge does **not** perform — migrations to apply,
new environment variables to set in the deploy target, a secret to rotate.

---

## 10. Release and deploy

Releases are tag-driven. `.github/workflows/release.yaml` triggers on `'*.*.*'`.

```
1. update CHANGELOG.md on master, then:
     git commit -m "docs(changelog): update changelog"

2. pick the next semver — patch for a fix, minor for a feature, and note that
   0.2.10 sorts *after* 0.2.9, not before

3. git tag 0.3.1 && git push origin 0.3.1
```

> **No `v` prefix.** The workflow's tag filter is `'*.*.*'` and every existing tag is bare
> (`0.3.0`). Pushing `v0.3.1` matches nothing, builds nothing, and reports no error.

The CHANGELOG entry is one heading per release, newest first, linking the tag and the PR,
with one bullet per commit subject:

```markdown
## [0.3.0](https://github.com/mini-apps-org/league-be/releases/tag/0.3.0) - 2026-11-10
- [#PR2](https://github.com/mini-apps-org/league-be/pull/2)
  - fix(cors): drive allowed origins from config, and narrow them per environment
```

The tag push then runs tests, authenticates to GCP by Workload Identity Federation (no
stored key), and pushes **one** image tagged verbatim from the tag:

```
asia-southeast1-docker.pkg.dev/vincent-mini-apps-project/league-repository/league-service:0.3.1
```

`latest` is deliberately never written (`flavor: latest=false`), so nothing can roll
forward by accident.

### Verify what is actually running

The tag is stamped into the binary at link time (`-ldflags -X …config.buildImage`) and read
back out at the health endpoint. It cannot be overridden by the environment, so this is
proof, not a report:

```
curl https://be.mvip-league.com/api/v1/health
{"ok":true,"env":"production","image":"0.3.1"}
```

### Building an image by hand

Only for a throwaway build; the tagged pipeline is the real path.

```
make ar-login                 # gcloud auth configure-docker
make image-push               # TAG defaults to the short HEAD sha
```

`--platform linux/amd64` is required, not optional: this machine is arm64 and the GKE
nodes are amd64. A natively-built image dies on the node with `exec format error`.

### Rolling out

Pushing the image does **not** restart anything — the cluster rollout lives outside this
repo. Record the exact command here once it settles, so the last step of a release is not
folklore.
