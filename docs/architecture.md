# Architecture: handler → service → repository → database

Conventions and the release steps live in
[`workflow.md`](workflow.md); SQL rules in
[`.claude/rules/database.md`](../.claude/rules/database.md).

`auth` is the only complete slice today. It is the template the next one copies.

---

## 1. The shape

```
HTTP request
   │
   ▼
cors → SecurityHeaders → HSTS (prod) → CSRF (unsafe methods) → RequireAuth (per route)
   │
   ▼
HANDLER      internal/auth/handlers.go       cookies, query params, status codes, redirects
   │  CallbackInput ↓        LoginResult ↑
SERVICE      internal/auth/services.go       business rules, orchestration, TTLs, crypto
   │  GoogleIdentityParams ↓     *User ↑
REPOSITORY   internal/auth/repositories.go   SQL, placeholders, row→struct, sentinel errors
   │  pgx
DATABASE     *pgxpool.Pool → PostgreSQL      schema in db/migrations/
```

One rule governs it: **each layer knows only the layer directly below it, and only
through an interface.** The handler holds a `Service`, the service holds a
`Repository`. Nothing holds a layer above it.

---

## 2. What each layer owns

| Layer | Owns | Must never |
|---|---|---|
| **Handler** | `*http.Request`/`ResponseWriter`, cookies, status codes, redirects, swagger annotations | Contain a business rule, build SQL, touch the pool |
| **Service** | Business rules, orchestration, time/TTL arithmetic, crypto, external providers | Take `*http.Request`, build SQL, choose a status code |
| **Repository** | Queries, placeholder args, row→struct, package sentinel errors | Decide HTTP status, apply a business rule, return a driver error upward |
| **Database** | Schema, constraints, indexes | — |

The two that are easiest to break:

- **The service never sees `*http.Request`.** `CompleteLogin(ctx, CallbackInput{…})`
  takes a struct the handler fills in — which is why it is testable without a server.
- **The repository never decides an HTTP status.** It returns `ErrNoSession`; the
  mapping to `40102` happens one layer up.

---

## 3. One struct family per boundary

The layering is typed, not remembered:

| File | Family | Example | Crosses |
|---|---|---|---|
| `model_handlers.go` | wire / JSON | `UserResponse`, `MeResponse` | handler ↔ browser |
| `model_services.go` | service boundary | `CallbackInput`, `LoginResult` | handler ↔ service |
| `models.go` | domain + db rows | `User`, `Session` (`db:"…"`) | service ↔ repository ↔ db |

A `db:"…"` struct is never serialized. `newUserResponse(u *User) UserResponse` is the
explicit translation, so a column added to `users` tomorrow does not appear in
`/api/v1/me` by itself.

`Service`, `Repository` and `IdentityProvider` are interfaces; the impls are unexported
and reached only through `NewService` / `NewRepo` / `NewHandler`.

---

## 4. Wiring

Everything is constructed once at startup, outermost-in. No globals, no service locator.

```
cmd/server/main.go
  config.Load()  →  db.Connect(ctx, cfg)  →  router.NewRouter(ctx)
                                               └─ registerAuthRoute   internal/platform/router/auth.go
                                                    NewRepo(pool)
                                                    NewService(cfg, google, repo, sealer)
                                                    NewHandler(cfg, service)
```

Imports point one way: `router → auth → platform/{config,db,handler,model} → {common,secret}`.
`platform` never imports a feature — the CSRF middleware is *given* the cookie name
(`middleware.NewCSRF(cfg, auth.CSRFCookieName(cfg))`) rather than importing `auth`.

`router` is the only package that knows a feature's constructors exist.

---

## 5. Trace: `GET /api/v1/me`

```
CSRF             GET is safe → pass through
RequireAuth      sid cookie → service.SessionUser(ctx, sid)
  SERVICE        repo.GetSession(ctx, sid, now-sessionTTL, now-idleTTL)
    REPOSITORY   SELECT … WHERE id=$1 AND created_at>$2 AND last_seen_at>$3
                 pgx.ErrNoRows → ErrNoSession
  SERVICE        repo.GetUserByID(…)  then  repo.UpdateSessionLastSeenAt(…)
RequireAuth      ok → user into ctx;  err → clear cookie, 401 40102
HANDLER          UserFrom(ctx) → ReturnWithJSON(200, SuccessResponse{Data: MeResponse{…}})
```

Note that `GetSession` takes two `time.Time` cutoffs, not TTLs: deciding they are
`now - sessionTTL` and `now - idleTTL` is the business rule, so it stays in the service.

---

## 6. Errors travel back the same path

```
REPOSITORY   pgx.ErrNoRows → ErrNoSession;  else fmt.Errorf("auth: get session: %w", err)
SERVICE      errors.Is(err, ErrNoSession) → handler.Error(handler.CodeSessionInvalid, err)
HANDLER      h.ReturnWithError(w, r, err)
             → {"error":{"code":"40102","message":"Your session has expired…"}}
```

The repository knows *which query* failed. The service knows what the failure *means*.
The handler knows neither. Anything unclassified becomes `50001` with the cause logged
and never serialized.

---

## 7. Adding a feature slice

Build inward-out — each layer compiles and is testable before the one above exists.

```
1  db/migrations/00002_players.sql
2  internal/players/models.go              Player{ db:"…" }
3  internal/players/repositories.go        Repository, NewRepo, the SQL
4  internal/players/model_services.go      service boundary types
5  internal/players/services.go            Service, NewService, the rules
6  internal/players/model_handlers.go      PlayerResponse, newPlayerResponse
7  internal/players/handlers.go            NewHandler, handlers + swagger annotations
8  internal/platform/router/players.go     registerPlayersRoute(ctx, api)
9  one call added to router.NewRouter
10 make swagger && make test
```

Pagination is a service concern: `params.Normalise()` runs before the query, the
repository selects `limit + 1`, and `model.NewCursorReponse(...)` builds the page.

---

## 8. Smells

| Smell | Instead |
|---|---|
| Handler takes `*pgxpool.Pool` | Handler → `Service` → `Repository` |
| Service takes `*http.Request` | An input struct in `model_services.go` |
| Repository returns `handler.Error(...)` | Return a sentinel; the service maps it |
| Repository returns `pgx.ErrNoRows` | Translate to a package sentinel |
| `db:"…"` struct returned as JSON | `newXResponse(...)` in `model_handlers.go` |
| `platform` imports a feature | Inject the value (see `NewCSRF`) |
| Values formatted into SQL | Placeholders + `args` — `database.md` §4 |
| Unbounded `SELECT` | `Normalise()` + keyset — `database.md` §9 |
