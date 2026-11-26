# Plan: League sessions API

## Goal

Serve the league-session admin routes from league-be: list, read, create, update and delete league
sessions, plus a per-week summary of a session's team matches.

A **league session** is a season of play, such as Fall 2026, stored in `league_sessions`. It is
not the login session (`users_sessions`, `internal/auth`).

## Sources

| What | Where |
|---|---|
| Schema | `db/migrations/00001_init.sql:68` (`league_sessions`), `:197` (`team_matches`) |
| Seed | `db/migrations/seeds/20261106_seed.sql:14` (Fall 2026) |
| Permissions | `session.read/create/update/delete` in `00001_init.sql:357`; SUPER_ADMIN has all |
| Slice to copy | `internal/roles/` and `internal/platform/router/role.go` |
| Conventions | `docs/workflow.md`, `.claude/rules/database.md`, `.claude/rules/naming.md` |

## Scope

**In:** the `league_sessions` schema changes, then `GET/POST /sessions`,
`GET/PATCH/DELETE /sessions/{id}` and `GET /sessions/{id}/weeks`.

**Out:**
- Frontend work. This plan covers league-be only.
- Divisions, teams, team matches and individual matches.
- Pagination. There are about three sessions a year, so the list is capped at 200 rows like `roles`.
- A league selector. The MVP has a single league; see decision D2.

## Decisions to confirm before coding

| # | Question | Recommendation |
|---|---|---|
| D1 | **Error code format.** BE sent 5-digit strings (`"40901"`); the client expects 6-digit numbers (`409001`). | **Done** on master in PR #8: `ErrorCode` is an `int64`, numbered status × 1000 + case. |
| D2 | **Where `league_id` comes from.** The create body has no `league_id`. | Single league: the service resolves the one `ACTIVE` league. If there isn't exactly one, it fails with `CodeInternal`, which is logged. Every query by id is scoped to that league. |
| D3 | **Default status.** The DB defaults to `IN_PROGRESS`; the admin form defaults to `DRAFT`. | Change the column default to `DRAFT` (Phase 0). The service still sets `DRAFT` explicitly when `status` is missing, so it does not depend on the default. |
| D4 | **One active session per league.** | Enforce it in the schema with a partial unique index (Phase 0), and deactivate the others in the same transaction as the write (Phase 4). |
| D5 | **Name and year rules in the schema.** | Add CHECKs as a backstop for the service validation: trimmed name 3–60 characters, `season_year` 2000–2100. Business rules live in Postgres. |
| D6 | **Deleting a session.** Submitted results are allowed to go with it. But `teams.division_id` cascades, so a team created in this session that also entered another session's division (`division_teams`) would be deleted from that session too. | **Block it** (built): the delete returns `409001` while such a team exists. Revisit if teams should outlive their home session. |
| D7 | **Package name.** `auth` already has a `Session` type. | `internal/sessions`, matching the routes and the `session.*` permission codes. Types stay named `LeagueSession…`, so logs and stack traces never mix them up with `auth.Session`. |

## Contract summary

All routes are under `/api/v1`, behind `RequireSession` and `RequirePermission`, with CSRF on the
unsafe methods.

| Route | Request | Success | Errors |
|---|---|---|---|
| `GET /sessions` | `q`, `session_type`, `season_year`, `status`, `sort` | 200, `count` = total | 400 bad param |
| `GET /sessions/{id}` | | 200 | 404 |
| `POST /sessions` | `{name, session_type, season_year, starts_on, ends_on, status?, is_active?}` | 201 | 400, 409 duplicate name |
| `PATCH /sessions/{id}` | any subset of the create body | 200 | 400, 404, 409 |
| `DELETE /sessions/{id}` | | 204 | 404 |
| `GET /sessions/{id}/weeks` | | 200 `[{week_no, match_count, submitted_count, first_scheduled_at}]` | 404 |

Bodies are snake_case, like `roles` and the `metadata` envelope. Dates are `YYYY-MM-DD`.
Responses use `handler.SuccessResponse[T]`; lists add `metadata` from `handler.NewMetadata`:

```json
{ "data": { "id": "uuid", "league_id": "uuid", "name": "Fall 2026", "session_type": "FALL",
  "season_year": 2026, "starts_on": "2026-09-01", "ends_on": "2026-12-20",
  "status": "IN_PROGRESS", "is_active": true, "week_count": 10,
  "has_submitted_results": true, "created_at": "…", "updated_at": "…" } }
```

- `sort`: `name`, `-name`, `starts_on`, `-starts_on` (the default), `season_year`, `-season_year`.
- `weekCount`: the number of distinct `team_matches.week_no` values across the session's divisions.
- `hasSubmittedResults`: true when any of those team matches has status `submitted`.

---

## Phase 0 — League session schema

Branch: `feat/league-sessions-schema`. Ship this first, as its own PR, so the API phases build on
the final table shape.

The `league_sessions` table already exists in `00001_init.sql`. This phase tightens it to the rules
the API enforces; it does not create a new table.

1. Add `db/migrations/2026/20261124-1-league-sessions-rules.sql`. Wrap it in `BEGIN; … COMMIT;` and make
   every statement re-runnable:
   ```sql
   BEGIN;

   -- D3: new sessions start as drafts; existing rows keep their status.
   ALTER TABLE league_sessions ALTER COLUMN status SET DEFAULT 'DRAFT';

   -- D4: at most one active session per league. The API deactivates the others first,
   -- in the same transaction, so this only fires on a bug or a manual write.
   CREATE UNIQUE INDEX IF NOT EXISTS league_sessions_one_active_per_league
     ON league_sessions (league_id) WHERE is_active;

   -- D5: backstops for the service validation.
   ALTER TABLE league_sessions DROP CONSTRAINT IF EXISTS league_sessions_name_length_check;
   ALTER TABLE league_sessions ADD CONSTRAINT league_sessions_name_length_check
     CHECK (char_length(btrim(name)) BETWEEN 3 AND 60);

   ALTER TABLE league_sessions DROP CONSTRAINT IF EXISTS league_sessions_season_year_check;
   ALTER TABLE league_sessions ADD CONSTRAINT league_sessions_season_year_check
     CHECK (season_year BETWEEN 2000 AND 2100);

   COMMIT;
   ```
2. Name the existing constraints the repository maps, and record them in the migration header
   comment:
   - `league_sessions_league_id_name_key` (from `UNIQUE (league_id, name)`) → duplicate name.
   - `league_sessions_check` (from `CHECK (ends_on >= starts_on)`) → validation.
   - Confirm both names with `\d league_sessions` before relying on them in Phase 4.
3. Update the seed: set `status` to `'IN_PROGRESS'` explicitly on Fall 2026 in
   `seeds/20261106_seed.sql`, since the column default no longer gives it that value.
4. Apply locally on a fresh database, then on the existing dev database:
   ```sh
   psql "$DATABASE_URL" -f db/migrations/2026/20261124-1-league-sessions-rules.sql
   psql "$DATABASE_URL" -f db/migrations/2026/20261124-1-league-sessions-rules.sql   # re-run is a no-op
   ```
   The seed has one active session with a valid name and year, so every constraint applies cleanly.
   On a database with dirty rows, find them first:
   ```sql
   SELECT league_id, count(*) FROM league_sessions WHERE is_active GROUP BY 1 HAVING count(*) > 1;
   SELECT id, name, season_year FROM league_sessions
   WHERE char_length(btrim(name)) NOT BETWEEN 3 AND 60 OR season_year NOT BETWEEN 2000 AND 2100;
   ```
5. Check each rule by hand inside a rolled-back transaction:
   - a second `is_active = true` row in the same league fails with `23505`
   - a name of `'  ab  '` fails with `23514`
   - `season_year = 1999` fails with `23514`
   - an insert with no `status` reads back `DRAFT`

**Done when:** the migration applies twice without error, the seed loads on a fresh database, and
the four checks above fail or pass as listed.

## Phase 1 — Swagger contract

Branch: `feat/league-sessions-swagger`. Ship this before any implementation, as its own PR, so the
contract is reviewed in `docs/swagger/swagger.yaml` before code is written against it.

Swagger is generated by `swag` from handler annotations (`make swagger`), so the contract is
written as Go: the wire types plus annotated handler stubs. Nothing is registered in the router,
so the running API does not change.

1. `internal/sessions/model_handlers.go`, wire types only:
   - `CreateLeagueSessionRequest`: `name`, `session_type`, `season_year`, `starts_on`, `ends_on`,
     `status?`, `is_active?`. Pointers mark the optional fields; dates are strings. The required
     fields carry `validate:"required"`, which only swag reads.
   - `UpdateLeagueSessionRequest`: every field a pointer.
   - `LeagueSessionResponse` and `LeagueSessionWeekResponse`, with **snake_case** json tags.
   - Give each field `example:"…"` and, for the enums, `enums:"SPRING,SUMMER,FALL"` and
     `enums:"DRAFT,IN_PROGRESS,COMPLETE,CANCELLED"`, plus `format:"date"` on the date strings, so
     the generated schema matches the contract summary above.
   - The mappers come in Phase 3, once the DB row type exists.
2. `internal/sessions/handlers.go`: `type Handler struct{ handler.Handler }` and one method
   per route, with an empty body and the full annotation block:
   - `ListLeagueSessions` — `@Param` for `q`, `session_type`, `season_year`, `status` and `sort`,
     with `Enums(…)` on the enum and sort params and `default(-starts_on)` on `sort`.
   - `GetLeagueSession`, `CreateLeagueSession`, `UpdateLeagueSession`, `DeleteLeagueSession` and
     `ListLeagueSessionWeeks`.
   - Each one has `@Summary`, `@Tags sessions`, `@Accept`/`@Produce`, `@Param`, `@Success`
     (`handler.SuccessResponse[…]`, 201 on create, 204 on delete), one `@Failure` per status
     with its codes in the current format, `@Security CSRFToken` on POST, PATCH and DELETE, and
     `@Router`.
   - `@Description` on create and update states the rules: name 3–60 characters after trimming,
     year 2000–2100, `ends_on` on or after `starts_on`, a missing `status` becomes `DRAFT`, and
     `is_active: true` deactivates the league's other sessions.
3. Run `make swagger`, then `make vet` (the stubs must compile).
4. Review the diff of `docs/swagger/swagger.yaml` against the contract summary: paths, params,
   request and response schemas, status codes. Fix the annotations, not the generated files.
5. Commit `internal/sessions/` and `docs/swagger/*` together.

**Done when:** the six `/sessions` operations show in Swagger UI under the `sessions` tag with the
schemas above, and `make test` passes.

From here on, the swagger is the contract: a later phase that needs a change to it updates the
annotations and regenerates in the same commit, and says so in its PR.

## Phase 2 — Align error codes — done

Shipped on master in PR #8 (`210ea28`, `5119e3d`): codes are 6-digit `int64`s, and each feature
maps its sentinels with a `[]handler.ErrorMapping` in its own `errors.go`. The Phase 1
`@Failure` lines already use the new numbers. Nothing left to do here.

## Phase 3 — Models (`internal/sessions/`) — done

Branch: `feat/league-sessions-api`. Service params live in `models.go`, as in `teams` and
`roles`; there is no `model_services.go`.

1. `models.go`:
   - `LeagueSession` (the DB row): `db:"…"` tags, dates as `time.Time`, `SeasonYear int16`
     (SMALLINT), plus `WeekCount int64` (a `count`) and `HasSubmittedResults bool`.
   - `LeagueSessionWeek`.
   - `LeagueSessionType` and `LeagueSessionStatus`, with their allowed-value slices. They mirror
     the CHECKs in `000001-init.sql`.
   - The sort fields `leagueSessionSortFields` and `defaultLeagueSessionSort` (`-starts_on`).
     `model.ParseSort` defaults to ascending, so the handler uses this value when `?sort=` is
     empty.
   - The rule limits: name 3–60, year 2000–2100, `leagueSessionListLimit` 200.
   - `ListLeagueSessionsServiceParams`, `CreateLeagueSessionServiceParams` and
     `UpdateLeagueSessionServiceParams` (every field a pointer).
2. `model_handlers.go`: the mappers `newLeagueSessionResponse` and `newLeagueSessionWeekResponse`.
   They are explicit rather than `common.CopyFieldsByName`, because dates are formatted with
   `time.DateOnly`.
3. `model_handlers_test.go`: the date format, and that every response field is filled.

## Phase 4 — Repository (`repositories.go`) — done

As built: `constraintErrors` maps constraint names to sentinels, as in `teams`; the activation
race (`league_sessions_one_active_per_league`) is `ErrLeagueSessionActiveConflict` → 409; and
`DeleteLeagueSession` refuses a session homing a team that plays in another session (D6),
inside the delete's transaction. A PATCH that 404s rolls back its deactivation.

Follow `.claude/rules/database.md`: an `args` slice, one argument per line, placeholders only,
wrapped errors, package sentinels, `rows.Err()`.

1. Sentinels: `ErrLeagueSessionNotFound`, `ErrDuplicateLeagueSessionName`,
   `ErrInvalidLeagueSession`, `ErrNoDefaultLeague`.
2. Map Postgres errors, by the constraint names confirmed in Phase 0:
   - `23505` on `league_sessions_league_id_name_key` → the duplicate sentinel.
   - `23514` (any CHECK from Phase 0 or `00001`) → the validation sentinel.
   - Reuse the `isUniqueViolation` pattern from `roles/repositories.go`.
3. `leagueSessionColumns` adds the two derived fields as correlated subqueries:
   ```sql
   (SELECT count(DISTINCT tm.week_no) FROM team_matches tm
      JOIN divisions d ON d.id = tm.division_id WHERE d.season_id = league_sessions.id) AS week_count,
   EXISTS (SELECT 1 FROM team_matches tm JOIN divisions d ON d.id = tm.division_id
      WHERE d.season_id = league_sessions.id AND tm.status = 'submitted') AS has_submitted_results
   ```
4. Methods:
   - `GetDefaultLeagueID(ctx)`: `SELECT id FROM leagues WHERE status = 'ACTIVE' LIMIT 2`. Exactly
     one row, or `ErrNoDefaultLeague`.
   - `ListLeagueSessions(ctx, leagueID, params)`:
     - Build the filters with the `add(clause, value)` helper.
     - `q` is an escaped `ILIKE`.
     - `ORDER BY` comes from a whitelist map keyed by the sort value, never from user text, with
       `id` as the tie-break.
     - `LIMIT leagueSessionListLimit` (200).
   - `GetLeagueSessionByID(ctx, leagueID, id)`.
   - `CreateLeagueSession(ctx, leagueID, params)` and `UpdateLeagueSession(ctx, leagueID, id, params)`:
     - Both run in one transaction (`pgPool.BeginTx`).
     - When `is_active` is true, first run
       `UPDATE league_sessions SET is_active = false, updated_at = now() WHERE league_id = $1 AND id <> $2 AND is_active`.
       It must come first, or the Phase 0 partial index rejects the write.
     - Then `INSERT … RETURNING` or `UPDATE … SET …, updated_at = now() RETURNING`.
     - Re-read through `GetLeagueSessionByID` inside the transaction, to fill the derived fields.
   - `DeleteLeagueSession(ctx, leagueID, id)`: zero `RowsAffected()` → not found.
   - `ListLeagueSessionWeeks(ctx, leagueID, id)`:
     - Check that the session exists first, so an empty list and a missing session stay distinct.
     - Then `GROUP BY tm.week_no ORDER BY tm.week_no` with `count(*)`,
       `count(*) FILTER (WHERE tm.status = 'submitted')` and `min(tm.scheduled_at)`.

## Phase 5 — Service (`services.go`) — done

`Service` is an interface and `service` the implementation, as in `roles`.

1. Every method starts with `leagueID, err := s.repo.GetDefaultLeagueID(ctx)` (D2).
2. List: validate the enums, the year range and the `sort` value; anything else returns
   `CodeValidationFailed`.
3. Create (the same rules as the Phase 0 CHECKs, so a bad request fails here with a clear message
   before it reaches the database):
   - Trim the name: 3–60 characters, measured with `utf8.RuneCountInString`.
   - Validate the type and the status; a missing status becomes `DRAFT` (D3).
   - The year is between 2000 and 2100.
   - Parse the dates as `YYYY-MM-DD`; `ends_on` must be on or after `starts_on`.
   - `is_active` defaults to false.
4. Update:
   - Load the current row, apply the non-nil fields, then run the same validation on the merged
     result. This catches a PATCH that moves only `ends_on` before `starts_on`.
   - An empty body returns `CodeValidationFailed`.
5. `errors.go` holds the sentinels and `leagueSessionErrorMappings`, a `[]handler.ErrorMapping`
   like `roleErrorMappings` in `internal/roles/errors.go`:
   - not found → `CodeNotFound`
   - duplicate → `CodeConflict`
   - validation → `CodeValidationFailed`
   - no default league → `CodeInternal`, logged
6. Validation messages are safe to show and in plain English; they're mainly for logs and swagger.

## Phase 6 — Handlers (`handlers.go`) and routes — done

As built: the route file is `internal/platform/router/session.go`. The contract gained `409` on
DELETE (D6) and a second `409` reason on POST and PATCH (concurrent activation); `swagger.*` was
regenerated in the same change.

1. Add the `service Service` field and `NewHandler` to the Phase 1 `Handler`, then fill in the
   stub bodies of `ListLeagueSessions`, `GetLeagueSession`, `CreateLeagueSession`,
   `UpdateLeagueSession`, `DeleteLeagueSession` and `ListLeagueSessionWeeks`.
   - They do HTTP only: query params or `h.DecodeJSON`, then the service, then
     `h.ReturnWithJSON` with `handler.SuccessResponse[T]`, adding `handler.NewMetadata(len(items))` on lists.
   - Create returns 201. Delete returns 204 with no body.
   - `DecodeJSON` already rejects unknown fields.
   - Leave the annotation blocks as they are; the behaviour must match them.
2. Add `PermissionFeatureSessions PermissionFeature = "SESSION"` in
   `internal/platform/config/globals_permissions.go`. It maps to the codes `session.read` and so on.
3. Add `internal/platform/router/session.go` with `registerLeagueSessionRoute`. It uses the
   same `guard(feature, next)` pattern as `role.go`. `/sessions/{id}/weeks` is a GET, so it needs
   `session.read`.
4. Call it from `NewRouter` after `registerRoleRoute`.
5. Run `make swagger`. `docs/swagger/*` must not change; a diff means the contract moved and needs
   the review described at the end of Phase 1.

## Phase 7 — Tests — done

As built: `services_test.go`, `handlers_test.go` and `router/session_test.go`. The router test runs
the real permission middleware with a fake session, and stops each request at a handler 400 so no
database is needed. Mutation-checked: guarding POST with the wrong feature, or making the default
sort ascending, fails the suite.

Stdlib `testing` with table cases and a fake `Repository`, so no database is needed. Each test
carries a comment saying what breaks in production if it fails.

1. **Service validation:**
   - name lengths 2, 3, 60 and 61, and whitespace-only
   - a bad type or status
   - years 1999 and 2101
   - a bad date format, and an end before the start
   - a missing status becomes `DRAFT`
2. **PATCH merge:** patching only `ends_on` to before the existing `starts_on` returns
   `CodeValidationFailed`; an empty patch returns `CodeValidationFailed`.
3. **Sort and filter parsing:** every allowed `sort` value is accepted, and `sort=created_at`
   returns `CodeValidationFailed`.
4. **Error mapping:** each sentinel maps to the right code and status, including a `23514` from the
   repository mapping to the validation sentinel.
5. **Router:** each route needs the right permission code for its method. A user with only
   `session.read` gets `403` on POST.

## Phase 8 — Verify end to end — done

As built: run against a scratch database from `000001-init.sql`, `20261124-1-league-sessions-rules.sql` and the 20261122 seed,
with `users_sessions` rows inserted for the seed's dev admin and dev captain instead of a browser
sign-in. Every check below passed, plus 401001/401002 without or with a bad `sid`, 403001/403002
on CSRF, and the live response keys matching the served swagger schemas.

1. Run `test -z "$(gofmt -l .)"`, `make vet` and `make test`.
2. Start the API with `make run`, get the `sid` and `csrf` cookies from a signed-in browser session,
   then check each of these with `curl`:
   - list, the filters, and each sort
   - creating "Spring 2027" with no `status`: it comes back `DRAFT`
   - creating it again (409)
   - a PATCH that sets `is_active: true`: Fall 2026 must become inactive
   - DELETE (204), then GET (404)
   - `/weeks` on the seeded session
3. A captain gets `200` on the reads and `403005` on POST, PATCH and DELETE: the seed grants
   CAPTAIN `session.read`.
4. Each response matches its schema in Swagger UI (`/swagger/index.html`).

## Phase 9 — Ship

1. Three PRs, in order: Phase 0 (schema), Phase 1 (swagger contract), Phases 3–8 (API).
2. Commits are scoped `migration`, `seed`, `swagger`, `sessions`, `router` and `config`, for example
   `feat(migration): enforce league session rules`,
   `docs(swagger): league sessions contract` and
   `feat(sessions): list, create, update and delete league sessions`.
3. The schema PR body lists the manual step: apply `2026/20261124-1-league-sessions-rules.sql` before
   deploying, after running the dirty-row queries from Phase 0 against production.
4. After the merge: a CHANGELOG entry and a minor tag (`0.10.0`; tags are at `0.9.0`).
5. Follow-up in **league-solution-design:** in `07-api-design.md`, change `/seasons` to `/sessions`
   and add the new routes; mark sessions as built in chapter 15.

## Risks

- **The Phase 0 constraints fail on dirty production rows.** Run the dirty-row queries first; the
  migration aborts as a whole, inside its transaction, if any row breaks a rule.
- **The default status changes to `DRAFT`.** Any insert that relied on the old `IN_PROGRESS`
  default, such as the seed, now creates a draft. The seed is updated in Phase 0; the API always
  sets `status` explicitly.
- **The derived fields' subqueries run per row.** This is fine at about three sessions a year and
  is covered by `team_matches_division_id_week_no_idx`. Revisit it if the list ever pages.
- **Swagger UI lists the session routes before they exist.** Between the Phase 1 merge and the
  API merge, calls to `/sessions` return 404. Keep that window short, or hold the Phase 1 PR open
  until the API PR is ready to follow.
- **Single-league resolution (D2)** must be replaced by real league scoping before a second
  league exists.
