# Rules: schema migrations

Applies to every change to the database schema: `db/migrations/000001-init.sql` and the
files under `db/migrations/<year>/`. There is no migration runner; files are applied with
`psql -f`, so their names and contents are the whole contract.

---

## 1. Every schema change is made in two places

| File | Brings | Read by |
|---|---|---|
| `db/migrations/000001-init.sql` | an **empty** database to the current schema | a new environment, a recreated dev database, CI |
| `db/migrations/<year>/<yyyymmdd>-<n>-<title>.sql` | an **existing** database from the previous schema to this one | every database already initialised |

`000001-init.sql` always describes the whole current schema — a reader should never need
to replay the dated files to know what a table looks like. The dated file is the step for
databases that ran an older `000001-init.sql`.

### Avoid
```sql
-- 000001-init.sql untouched; the column exists only in a migration, so a fresh
-- database built from init is missing it until someone remembers to replay 2026/.
```

### Prefer
```sql
-- 000001-init.sql
CREATE TABLE IF NOT EXISTS roles (
	…
	is_deleted   BOOLEAN     NOT NULL DEFAULT false,
	…
);

-- db/migrations/2026/20261122-1-add-is-deleted-for-roles.sql
BEGIN;
ALTER TABLE roles ADD COLUMN IF NOT EXISTS is_deleted BOOLEAN NOT NULL DEFAULT false;
COMMIT;
```

---

## 2. Name: `db/migrations/<year>/<yyyymmdd>-<n>-<title>.sql`

```
db/migrations/2026/20261122-1-add-is-deleted-for-roles.sql
db/migrations/2026/20261122-2-add-is-deleted-for-team.sql
```

| Part | Rule |
|---|---|
| `<year>` folder | the year of `<yyyymmdd>` |
| `<yyyymmdd>` | the day the migration is written |
| `<n>` | 1, 2, 3 … restarting each day, in the order they must run |
| `<title>` | lower-case kebab-case, starting with a verb: `add-…`, `drop-…`, `rename-…` |

Files run in name order. A day holds at most 9 (`-10-` sorts before `-2-`); split work
across days rather than going past that.

---

## 3. A migration is safe to run twice

It may run on a database built from the current `000001-init.sql`, where its change is
already present, or be re-run after a failure. Both must be no-ops:

- Wrap it in `BEGIN; … COMMIT;` so a failure leaves nothing half-applied.
- `ADD COLUMN IF NOT EXISTS`, `CREATE TABLE/INDEX IF NOT EXISTS`, `DROP … IF EXISTS`.
- A `NOT NULL` column on a table with rows needs a `DEFAULT` in the same statement.
- Data fixes are written so a second run matches nothing (`WHERE new_column IS NULL`).

Test both paths on a throwaway database before committing: init alone, and the previous
`000001-init.sql` (from `git show HEAD:…`) followed by the migration, twice.

---

## 4. `000001-init.sql` stays runnable top to bottom

- Create a table after every table it references: a foreign key to a table defined
  further down fails with `relation "…" does not exist` and rolls the whole file back.
- Keep each constraint name the code maps errors from (`constraintErrors` in
  `internal/*/repositories*.go`); renaming one silently turns a 409 into a 500.
- An index or constraint the code relies on (`users_sessions_user_id_key`, used by
  `ON CONFLICT (user_id)`) belongs in init, not only in a dated file.

---

## 5. Order of application

```
db/migrations/000001-init.sql            -- empty database only
db/migrations/<year>/*.sql              -- existing databases, in name order
db/migrations/seeds/<yyyymmdd>-seed.sql -- development data, last: the LATEST seed only
```

List the migrations a PR adds, in order, in its description: nothing applies them
automatically.

---

## 6. Seeds

- **A new checkout applies only the latest seed** — the newest `<yyyymmdd>-seed.sql`.
  Older seeds match older schemas and stay for history; each says so in its header.
- **A schema change that breaks the latest seed gets a new seed** dated the day of the
  change, rather than an edit to a seed someone may already rely on.
- **Every seed is re-runnable.** It starts with `TRUNCATE … RESTART IDENTITY CASCADE` of
  the league data it inserts, so applying it again resets the data instead of failing on
  duplicates. List only tables that exist in its schema: `TRUNCATE` has no `IF EXISTS`.
- **A seed never truncates what `000001-init.sql` fills** (`roles`, `permissions`,
  `roles_permissions`, `race_chart`), **nor `users`** — that would delete real accounts,
  their sessions and any role granted to them. It removes only its own dev accounts, by
  `google_sub`.
