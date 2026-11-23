# Testing the API on localhost

From a fresh checkout to an authenticated request in Postman or curl. How the service is
built is [`architecture.md`](architecture.md); schema and seed rules are
[`.claude/rules/migrations.md`](../.claude/rules/migrations.md).

```
1 configure .env  →  2 create the database  →  3 run the API  →  4 sign in in a browser
→  5 grant yourself a role  →  6 call the API from Postman or curl
```

---

## 1. Prerequisites

| Tool | Why |
|---|---|
| Go (the version in `go.mod`) | build and run |
| PostgreSQL and `psql` | the database; there is no migration runner, files are applied with `psql` |
| [`air`](https://github.com/air-verse/air) (optional) | `make run` reloads on save; `make api` runs without it |
| A Google OAuth client | sign-in is Google only — see below |
| Postman (optional) | or curl |

**Google OAuth client.** In Google Cloud Console → APIs & Services → Credentials, create an
*OAuth client ID* of type **Web application** and add this **authorised redirect URI**,
exactly:

```
http://localhost:9200/api/v1/auth/google/callback
```

Copy its client ID and secret into `.env` (next step).

---

## 2. Configure `.env`

Copy `.env.example` to `.env` and fill it in. The process refuses to start while a
required value is missing.

| Variable | Local value | Required |
|---|---|---|
| `APP_ENV` | `development` (the default when empty) | — |
| `DB_HOST` / `DB_PORT` / `DB_USER` / `DB_PASSWORD` / `DB_NAME` | your local Postgres, e.g. `localhost` / `5432` / `postgres` / … / `league-backend` | name and password |
| `DATABASE_URL` | leave empty locally — when set it wins over every `DB_*` | — |
| `GOOGLE_OAUTH_CLIENT_ID`, `GOOGLE_OAUTH_CLIENT_SECRET` | from step 1 | **yes** |
| `ENCRYPTION_KEY` | `openssl rand -base64 32` | **yes** |
| `FRONTEND_URL` | where sign-in redirects afterwards; the SPA need not be running | — |
| `SESSION_TTL`, `SESSION_IDLE_TTL` | defaults `12h` / `30m` | — |
| `LOG_LEVEL` | empty → `debug` in development, so runtime-built SQL is logged | — |

---

## 3. Create the database

```bash
set -a; . ./.env; set +a
export PGPASSWORD="$DB_PASSWORD"
PSQL="psql -h $DB_HOST -p $DB_PORT -U $DB_USER -v ON_ERROR_STOP=1"

$PSQL -d postgres -c "CREATE DATABASE \"$DB_NAME\""                     # once

$PSQL -d "$DB_NAME" -f db/migrations/000001-init.sql                     # 1. the whole schema
for f in db/migrations/[0-9][0-9][0-9][0-9]/*.sql; do                    # 2. dated migrations, in name order
  $PSQL -d "$DB_NAME" -f "$f"
done
$PSQL -d "$DB_NAME" -f "$(ls db/migrations/seeds/*.sql | sort | tail -1)" # 3. the LATEST seed only
```

- **Fresh database:** `000001-init.sql` already contains everything, so the dated migrations
  change nothing — run them anyway; it is the same command for everyone.
- **Existing database:** skip init; run only the dated migrations you have not applied, then
  the seed.
- **Only the latest seed.** Older seeds match older schemas.
- **Reset the development data** at any time by re-running the latest seed: it truncates the
  league data first. It leaves roles, permissions and real accounts alone.

The seed creates one league, one division (`8-Ball Open`), four teams (`03207`, `03209`,
`03211`, `03214`) of seven or eight players, and two fixtures. Players have no member id
(`''`) — assign them by hand.

---

## 4. Run the API

```bash
make run          # with live reload (air)
make api          # or plain go run
```

| URL | What |
|---|---|
| `http://localhost:9200/api/v1/health` | liveness — `{"ok":true,…}` without signing in |
| `http://localhost:9200/swagger/index.html` | every endpoint, its parameters and error codes |
| `http://localhost:9091/metrics` | Prometheus metrics, on their own port |

```bash
curl -s localhost:9200/api/v1/health
```

---

## 5. Sign in

The API has no username/password and no bearer tokens. Sign-in is Google OAuth handled by
the API itself, which ends by setting two cookies. Postman's built-in "OAuth 2.0" helper does
not apply — sign in once in a browser and reuse the cookies.

1. Open **`http://localhost:9200/api/v1/auth/google/login`** and sign in with Google.
2. You are redirected to `FRONTEND_URL`. If nothing runs there, the page fails to load —
   that is fine; the cookies are already set on `localhost`.
3. DevTools → **Application** → **Cookies** → `http://localhost:9200`, copy:

| Cookie | Role | Send it as |
|---|---|---|
| `sid` | the session | `Cookie` header, every authenticated request |
| `csrf` | double-submit token | `Cookie` header, **and** the `X-CSRF-Token` header on every `POST`, `PATCH`, `PUT`, `DELETE` |

Outside production the names are bare. In production they are `__Host-sid` and
`__Secure-csrf`, and `Secure`.

**One session per user.** Signing in again anywhere replaces the session — the old `sid`
then answers `401002`. The session also ends after `SESSION_IDLE_TTL` (30 min) without a
request, or `SESSION_TTL` (12 h) in total.

---

## 6. Give yourself a role

A new account gets `PLAYER`: it can read, not change. To exercise every endpoint, make
yourself `SUPER_ADMIN`:

```sql
INSERT INTO users_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'you@gmail.com' AND r.code = 'SUPER_ADMIN'
ON CONFLICT DO NOTHING;
```

Roles are read on every request, so this applies without signing in again. Check with
`GET /api/v1/me`, which lists your roles and permissions.

**Testing as a captain.** The seed's `dev-admin` and `dev-captain` accounts cannot sign in —
no Google account matches them. To test captain rules, link your own account to a player
and make that player a team's captain:

```sql
UPDATE players SET user_id = (SELECT id FROM users WHERE email = 'you@gmail.com')
WHERE full_name = 'Le Minh Quan';                       -- captain of 03209 in the seed

INSERT INTO users_roles (user_id, role_id)
SELECT u.id, r.id FROM users u, roles r
WHERE u.email = 'you@gmail.com' AND r.code = 'CAPTAIN'
ON CONFLICT DO NOTHING;

DELETE FROM users_roles                                 -- drop SUPER_ADMIN while testing as captain
WHERE user_id = (SELECT id FROM users WHERE email = 'you@gmail.com')
  AND role_id = (SELECT id FROM roles WHERE code = 'SUPER_ADMIN');
```

---

## 7. Postman

1. **Import** → choose `docs/swagger/swagger.json`. Every endpoint arrives with its
   parameters and example bodies. Regenerate the file first with `make swagger` if the code
   changed.
2. On the collection, **Variables** tab — add:

   | Variable | Value |
   |---|---|
   | `baseUrl` | `http://localhost:9200/api/v1` |
   | `sid` | the `sid` cookie from step 5 |
   | `csrf` | the `csrf` cookie from step 5 |

3. On the collection, **Scripts → Pre-request**, paste — every request then carries the
   cookies, and every write the CSRF header:

   ```js
   const sid = pm.collectionVariables.get('sid');
   const csrf = pm.collectionVariables.get('csrf');
   pm.request.headers.upsert({ key: 'Cookie', value: `sid=${sid}; csrf=${csrf}` });

   // Writes must echo the csrf cookie in a header (double-submit).
   if (!['GET', 'HEAD', 'OPTIONS'].includes(pm.request.method)) {
     pm.request.headers.upsert({ key: 'X-CSRF-Token', value: csrf });
   }
   ```

4. Send `GET {{baseUrl}}/me`. A `200` with your email means you are in.

When the session expires, sign in again in the browser and paste the new `sid` and `csrf`
— nothing else changes.

---

## 8. curl

```bash
BASE=http://localhost:9200/api/v1
SID='…'    # from step 5
CSRF='…'
AUTH=(-H "Cookie: sid=$SID; csrf=$CSRF")
WRITE=(-H "Cookie: sid=$SID; csrf=$CSRF" -H "X-CSRF-Token: $CSRF" -H "Content-Type: application/json")

curl -s "${AUTH[@]}" "$BASE/me"
```

---

## 9. A walk through the teams API

Ids are uuids; read them from the previous response or from the database
(`SELECT id, name FROM divisions;`).

```bash
# Teams, optionally in one division
curl -s "${AUTH[@]}" "$BASE/teams?division_id=$DIVISION_ID"

# Create a team (team_code is XXyyA: region, host location, team sequence)
curl -s "${WRITE[@]}" -X POST "$BASE/teams" \
  -d '{"team_code": "41503", "name": "Cue Masters"}'

# Enter it into a division — a team plays a season through a division entry
curl -s "${WRITE[@]}" -X POST "$BASE/teams/$TEAM_ID/divisions" \
  -d "{\"division_id\": \"$DIVISION_ID\"}"

# Roster in that division; add a player
curl -s "${AUTH[@]}" "$BASE/teams-roster/$TEAM_ID?division_id=$DIVISION_ID"
curl -s "${WRITE[@]}" -X POST "$BASE/teams-roster/$TEAM_ID?division_id=$DIVISION_ID" \
  -d "{\"player_id\": \"$PLAYER_ID\"}"

# Transfer the player to another team (same division unless division_id is given)
curl -s "${WRITE[@]}" -X PATCH "$BASE/teams-roster/$TEAM_ID/$PLAYER_ID?division_id=$DIVISION_ID" \
  -d "{\"team_id\": \"$OTHER_TEAM_ID\"}"

# Soft delete, then permanently delete (only a soft-deleted team; never one with fixtures)
curl -s "${WRITE[@]}" -X DELETE "$BASE/teams/$TEAM_ID"
curl -s "${WRITE[@]}" -X DELETE "$BASE/teams/$TEAM_ID/permanent"
```

The full list, with every parameter and error, is the Swagger UI.

---

## 10. When a request fails

Every error has the same body — `{"error": {"code": 401002, "message": "…"}}` — and the
server log line carries the cause under the same `request_id`.

| Code | Meaning | Fix |
|---|---|---|
| `400001` | a parameter or body field is invalid | the message names the field |
| `401001` | no `sid` cookie | check the `Cookie` header / pre-request script |
| `401002` | session expired or replaced | sign in again, copy the new cookies |
| `403001` | no `csrf` cookie on a write | send the `csrf` cookie too |
| `403002` | `X-CSRF-Token` differs from the `csrf` cookie | both must hold the same value |
| `403005` | your role lacks the permission | step 6 |
| `404001` | the id does not exist, or the row is soft-deleted | |
| `409001` | conflicts with existing data (duplicate team code, team has fixtures, …) | the message says which |
| `500001` | unexpected server error | read the server log for that `request_id` |

| Symptom | Cause |
|---|---|
| `config: GOOGLE_OAUTH_CLIENT_ID is required` at start | `.env` not filled in, or not loaded (`make run` loads it; a bare `go run` does not) |
| Google shows `redirect_uri_mismatch` | the redirect URI in step 1 differs from `http://localhost:9200/api/v1/auth/google/callback` |
| Sign-in loops back to `?error=login_failed` | took more than 10 minutes on Google's screen, or blocked cookies on `localhost` |
| `ERROR: relation "…" does not exist` in step 3 | a migration ran out of order — run init, then the dated files in name order |
| CORS errors | browsers only; Postman and curl do not apply CORS |
