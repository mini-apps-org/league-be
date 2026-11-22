# Changelog

## [0.8.0](https://github.com/mini-apps-org/league-be/releases/tag/0.8.0) - 2026-11-22

- [#PR7](https://github.com/mini-apps-org/league-be/pull/PR7)
  - feat(team): add the teams API — create a team in its home division, update it, and enter it into more divisions
  - feat(roster): add per-division rosters at `/teams-roster/{id}` — add, transfer and remove players; a captain changes only their own team
  - feat(team): filter, sort and cursor-page the team and roster lists with `q`, `sort`, `limit` and `cursor`
  - feat(team): soft-delete a team with `DELETE /teams/{id}`, then remove it for good with `DELETE /teams/{id}/permanent`
  - feat(team): bulk-delete up to 100 teams with `DELETE /teams` and `DELETE /teams/permanent`, reporting each id that failed
  - feat(roles): soft-delete roles and add `DELETE /roles/{id}/permanent` — a deleted role stops granting permissions at once
  - refactor(response): success bodies are now `{ data, metadata }`, with `count`, `total`, `has_more` and `next_cursor` on lists — breaking: top-level `count` and `session_id` are gone
  - feat(logger): inject `*slog.Logger` into services and repositories, and log runtime-built team SQL at debug level
  - refactor(db): rename the schema to `000001-init.sql` and add dated migrations under `db/migrations/2026/`

## [0.7.0](https://github.com/mini-apps-org/league-be/releases/tag/0.7.0) - 2026-11-19

- [#PR6](https://github.com/mini-apps-org/league-be/pull/PR6)
  - docs: service overview diagrams
  - feat(metrics): HTTP request metrics middleware
  - feat(metrics): serve /metrics on its own port and read server timeouts from config

## [0.6.0](https://github.com/mini-apps-org/league-be/releases/tag/0.6.0) - 2026-11-18

- [#PR5](https://github.com/mini-apps-org/league-be/pull/PR5)
  - feat(trace): add trace id for server

## [0.5.0](https://github.com/mini-apps-org/league-be/releases/tag/0.5.0) - 2026-11-15

- [#PR4](https://github.com/mini-apps-org/league-be/pull/PR4)
  - feat(authorization): seed role `PLAYER` for new user
  - fix(golangci-lint): ignore lint `S1016`
  - docs(claude): naming convention rule
  - refactor: clearer names and leaner comments in committed code
  - feat(roles-permissions): RBAC `roles/permissions` catalog and permission middleware
  - feat(auth): one session per user (`ReplaceSession`)
  - feat(authorization): add `roles/permissions` when get user info
  - refactor(database): share pgx query helpers in `platform/handler/repo`

## [0.4.0](https://github.com/mini-apps-org/league-be/releases/tag/0.4.0) - 2026-11-12

- refactor(middleware): re-structure middleware and add `go vet` in ci/cd pipeline
- refactor(router): extract `handleSwagger` and hoist route consts
- feat(logger): setup logger for server
- refactor(config): remove dead logrus config scaffolding
- docs: add `pull_request_template`/`architecture`/`tips`/`workflow` for codebase

## [0.3.0](https://github.com/mini-apps-org/league-be/releases/tag/0.3.0) - 2026-11-10
- [#PR2](https://github.com/mini-apps-org/league-be/pull/2)
  - docs(auth): rewrite the `README` against the current code
  - fix(cors): drive allowed origins from config, and narrow them per environment
  - refactor(auth): move the session sweep onto `Service` and share `RandomToken`
  - feat(db): add `leagues`, `operators` and `roster history`, and rework the `permission` catalog

## [0.2.10](https://github.com/mini-apps-org/league-be/releases/tag/0.2.10) - 2026-11-08

- fix(auth): scope cookies to the registrable domain so the SPA on `www.` can read the `csrf` token
- refactor(auth): derive cookie `Path` and `Domain` from the cookie name so the two cannot disagree
- refactor(auth): rename the cookie helpers and group their inputs into `cookieParams`

## [0.2.7](https://github.com/mini-apps-org/league-be/releases/tag/0.2.7) - 2026-11-08
- [#PR1](https://github.com/mini-apps-org/league-be/pull/1)
  - fix(auth): stop sending `__Host-` on cookies that carry a `Domain` or a non-root `Path`, which browsers discard
  - fix(auth): give the `csrf` cookie a `Domain` so a frontend on another host can read it
  - feat(config): replace `FRONTEND_URL` with `app_base_url.frontend` per environment

## [0.1.3](https://github.com/mini-apps-org/league-be/releases/tag/0.1.3) - 2026-11-07

- feat(health): answer the health check at `/api/v1` as well as `/api/v1/health`

## [0.1.1](https://github.com/mini-apps-org/league-be/releases/tag/0.1.1) - 2026-11-07

- fix(config): point the Google OAuth redirect at `be.mvip-league.com` for `staging` and `production` environment

## [0.1.0](https://github.com/mini-apps-org/league-be/releases/tag/0.1.0) - 2026-11-06

- feat(auth): add Google OAuth sign-in with server-side sessions and a daily expiry sweep
- feat(db): add the league and RBAC schema, business rules and development seed data
- feat(config): add embedded per-environment YAML config with an environment overlay
- feat(db): add the pgx connection pool
- feat(handler): add the 5-digit error-code envelope and strict JSON request decoding
- feat(middleware): add double-submit CSRF and always-on security headers
- feat(platform): add generic keyset cursor pagination
- feat(secret): add an AES-256-GCM box for sealed values
- refactor(database): collect query arguments into an `args` slice, and document the convention
- chore: add the Go module, Makefile, distroless image and CI/release workflows
