# Rules: soft delete and permanent delete

Applies to every entity with a DELETE endpoint (today `teams` and `roles`), and to any
query that reads a table with an `is_deleted` column. Add the column to an entity in the
same migration that gives it a DELETE endpoint, not before.

---

## 1. Two endpoints, same permission

| Route | Effect | Permission |
|---|---|---|
| `DELETE /{entity}/{id}` | `UPDATE … SET is_deleted = true` → 204 | `<feature>.delete` |
| `DELETE /{entity}/{id}/permanent` | `DELETE … WHERE id = $1 AND is_deleted` → 204 | `<feature>.delete` |

There is no separate permission for a permanent delete — see `permissions.md`. What stops
an accidental one is the order: a permanent delete only matches a row that is already
soft-deleted, and answers 409 otherwise.

### Avoid
```go
const query = `DELETE FROM teams WHERE id = $1`
```

### Prefer
```go
const query = `DELETE FROM teams WHERE id = $1 AND is_deleted`
...
if tag.RowsAffected() > 0 {
	return nil
}
return r.missingOrLiveTeamError(ctx, id) // ErrTeamNotFound or ErrTeamNotSoftDeleted
```

---

## 2. Every read hides deleted rows

`GetXByID`, `ListX` and `UpdateX` filter `NOT is_deleted`, so a soft-deleted row is a 404
everywhere except the permanent-delete route. A soft delete of an already deleted row is
also a 404.

---

## 3. Writes that reference the entity check it in SQL

An `INSERT` that points at a soft-deletable row checks it in the same statement, so no
request can attach new data to a deleted row:

```sql
INSERT INTO team_rosters (division_id, team_id, player_id)
SELECT $1::uuid, $2::uuid, $3::uuid
WHERE EXISTS (SELECT 1 FROM teams WHERE id = $2 AND NOT is_deleted)
```

No rows affected → the parent's not-found sentinel.

---

## 4. A soft delete must take effect where it matters

Decide what "deleted" means for everything that reads the row, and do it in the same
statement or transaction:

- A soft-deleted **role** grants nothing: every permission lookup joins
  `roles … AND NOT is_deleted` (`permissions.GetPermissionCodesByUserID`,
  `auth.GetUserByID`, and the sign-up grant in `auth.UpsertGoogleIdentity`). A role check
  that forgets this keeps granting access after the role is "deleted".
- A soft-deleted **team** closes its active roster entries, so its players can join
  another team in the division.

---

## 5. Unique codes stay taken

A soft-deleted row keeps its unique values (`teams.team_number`, `roles.code`). Do not
make those unique indexes partial on `NOT is_deleted`: a code must never name two things
while the first still exists. It is free again only after a permanent delete.
