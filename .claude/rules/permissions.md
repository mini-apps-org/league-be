# Rules: permissions

Applies to every migration or seed that adds a permission, grants one to a role, or adds a
`config.PermissionFeature`. Permission codes are authorization contracts: the frontend and
`RequirePermission` both depend on them, so the catalog is kept deliberately small.

---

## 1. Four actions, no others

A permission code is `<feature>.<action>`, and the action is one of exactly four:

| Action | HTTP methods (`config.HTTPMethodToPermissionAction`) | Covers |
|---|---|---|
| `read` | `GET` | listing and getting one |
| `create` | `POST` | creating |
| `update` | `PUT`, `PATCH` | editing, including state changes such as a transfer |
| `delete` | `DELETE` | removing |

### Avoid
```sql
INSERT INTO permissions (code, name, description) VALUES
	('roster.manage_any', 'Manage any roster', '...'),
	('roster.list',       'List rosters',      '...');
```

### Prefer
```sql
INSERT INTO permissions (code, name, description) VALUES
	('roster.read',   'View rosters',   'See which players are on a team.'),
	('roster.create', 'Create rosters', 'Add a player to a team roster.'),
	('roster.update', 'Update rosters', 'Edit a roster entry.'),
	('roster.delete', 'Delete rosters', 'Remove a player from a roster.');
```

Viewing is `read`, never `list` or `view`: `GET` maps to `read`, and a code that no method
maps to can never be granted by `RequirePermission`. `player.change_skill_level` in
`00001_init.sql` predates this rule; do not add more like it.

---

## 2. Scope is a service rule, not a new code

"Only their own team" or "any team" is not a fifth action. Decide it in the service from
who the user is and the four actions they already hold.

### Avoid
```go
if slices.Contains(codes, "roster.manage_any") { … }
```

### Prefer
```go
// A user who may update any team may change any roster; anyone else only a team they captain.
if slices.Contains(codes, config.GetPermissionByFeatureAndAction(config.PermissionFeatureTeams, config.PermissionActionUpdate)) {
	return true, nil
}
```

Build codes with `config.GetPermissionByFeatureAndAction`, not string literals, so a code
the router cannot produce cannot be checked either.

---

## 3. A new feature seeds all four, in one migration

Add the `config.PermissionFeature` constant (upper case, singular: `TEAM`, `ROSTER`) and,
in the same migration, all four codes in lower case (`team.read` … `team.delete`). Then:

1. Rerun the `SUPER_ADMIN` grant — it receives every permission, and the grant in
   `00001_init.sql` only covered the codes that existed then.
2. Grant other roles by explicit `(role code, permission code)` pairs, so the matrix stays
   reviewable in the diff.

```sql
INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r CROSS JOIN permissions p
WHERE r.code = 'SUPER_ADMIN'
ON CONFLICT (role_id, permission_id) DO NOTHING;

INSERT INTO roles_permissions (role_id, permission_id)
SELECT r.id, p.id
FROM roles r
JOIN permissions p ON (r.code, p.code) IN (
	('CAPTAIN', 'roster.create'),
	('CAPTAIN', 'roster.update')
)
ON CONFLICT (role_id, permission_id) DO NOTHING;
```

---

## 4. Re-runnable, never renamed

- Every permission and grant `INSERT` ends in `ON CONFLICT … DO NOTHING`, so a migration
  or seed can be re-read top to bottom.
- Never rename or re-purpose a code. Change its `name` and `description` freely; to change
  what a code means, add the new one and migrate grants to it.
- Never delete a code a role holds without removing the grant in the same migration —
  `roles_permissions` cascades, so a role would silently lose access.
