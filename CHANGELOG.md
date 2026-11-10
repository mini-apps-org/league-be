# Changelog

## [TBD](https://github.com/mini-apps-org/league-be/releases/tag/0.2.10) - 2026-11-10

- feat(db): add `leagues` as the root of the hierarchy, with `league_operators` scoping who may run which league
- feat(db): add `team_rosters` as roster history, so a transfer preserves the match history it played in
- feat(db): rename `seasons` to `league_sessions` with an explicit `session_type` and `season_year`
- feat(db): rework the permission catalog into an `entity.action` matrix and rename `ADMIN` to `SUPER_ADMIN`
- fix(db): rename `sessions` to `users_sessions` and point the auth queries at it
- fix(db): grant the dev admin `SUPER_ADMIN` in the seed, which silently granted nothing after the rename
- fix(db): restore the `>= 0` floors on `players_stats` counters and constrain `league_sessions.status`
- fix(cors): name the allowed origins per environment, so production no longer accepts credentialed reads from `*.vercel.app` or plain-HTTP `localhost`
- fix(config): refuse an empty `domains.allowed_origins` at startup instead of falling open to every origin
- refactor(auth): move the session expiry sweep onto `Service` and share `RandomToken` via `internal/common`
- chore(db): make the whole migration re-runnable with `IF NOT EXISTS` and `ON CONFLICT DO NOTHING`
- docs(auth): rewrite `internal/auth/README.md` against the current code

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
