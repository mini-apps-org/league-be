# Changelog

## master - 2026-11-12

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
