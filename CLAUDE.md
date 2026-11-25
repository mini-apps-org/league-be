# league-be

## Documentation

- `docs/overview.md` — the service from the outside: what it talks to, the request
  pipeline, startup and shutdown, and the release path, as Mermaid diagrams.
- `docs/architecture.md` — the layering: handler → service → repository → database,
  the boundary types, the request traces and the composition root. Read it before
  adding a feature slice.
- `docs/data-model.md` — accounts, members (players) and teams: how they relate across
  divisions and seasons, the ID formats, and who may change a roster.
- `docs/local-testing.md` — running the API on localhost and calling it: `.env`, the
  database, Google sign-in, granting yourself a role, Postman and curl, error codes.
- `docs/transactions.md` — running several repository calls as one unit from a service:
  `db.Transactor`, `txCtx`, what rolls back, and when not to use it.
- `docs/workflow.md` — package layout, errors, HTTP, config, migrations and tests,
  then the step-by-step path from a feature branch to a deployed image.

## Conventions

When writing or editing any `internal/*/repositories.go` — the only layer that
speaks SQL — follow @.claude/rules/database.md

When naming any function, method, type, constant or variable, follow
@.claude/rules/naming.md

When changing the database schema — `db/migrations/000001-init.sql` or a file under
`db/migrations/<year>/` — follow @.claude/rules/migrations.md
When adding or granting a permission in a migration or seed, follow
@.claude/rules/permissions.md

When adding a DELETE endpoint, or any query that reads a table with `is_deleted`, follow
@.claude/rules/soft-delete.md

## CodeGraph — preferred search tool

**Prefer CodeGraph over `grep`/`find`/`Read` for locating symbols, tracing call flows, and exploring file structure.** CodeGraph is the pre-built AST index — faster, more accurate, and cheaper than file scans.

| Task                                 | Preferred tool      |
| ------------------------------------ | ------------------- |
| Find where a symbol is defined       | `codegraph_search`  |
| Find files under a path              | `codegraph_files`   |
| Understand an area / task context    | `codegraph_context` |
| Trace how X reaches Y                | `codegraph_trace`   |
| What calls function F?               | `codegraph_callers` |
| What does F call?                    | `codegraph_callees` |
| What would break if I change Z?      | `codegraph_impact`  |
| Read several related symbols at once | `codegraph_explore` |

Fall back to `grep`/`Read` only for **literal text** (log strings, comments, UI copy) or when codegraph reports a file as stale.

### Comments
Use comments sparingly. Only comment complex code where the logic isn't self-evident. Prefer clear variable names and simple logic over explanatory comments.