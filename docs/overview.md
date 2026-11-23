# Overview: how the service runs

The picture from the outside in. How one feature slice is layered inside is
[`architecture.md`](architecture.md); the Google login handshake is
[`internal/auth/README.md`](../internal/auth/README.md) §2.

---

## 1. What it talks to

```mermaid
flowchart LR
    spa["Browser / SPA<br/>mvip-league.com"]
    ingress["Ingress<br/>be.mvip-league.com"]
    prom["Prometheus"]

    subgraph pod["league-service pod"]
        api["API listener<br/>server.port: 5001 (9200 in dev)"]
        metrics["Metrics listener<br/>server.metrics_port: 9091"]
    end

    pg[("PostgreSQL")]
    google["Google OIDC"]

    spa -- "HTTPS, sid + csrf cookies" --> ingress --> api
    prom -- "GET /metrics" --> metrics
    api -- "pgxpool" --> pg
    api -- "discovery, token exchange, JWKS" --> google
```

- `/metrics` has its own port so the Service behind the public Ingress never exposes it;
  `config.Load()` refuses to start if the two ports are equal.
- The session lives in Postgres (`users_sessions`), so any number of pods can serve the same user.

---

## 2. A request through the API listener

```mermaid
flowchart TD
    req["HTTP request"] --> cors["CORS<br/>one allowed origin, credentials on"]
    cors --> trace["Trace<br/>X-Request-Id kept or minted"]
    trace --> metrics["RecordRequestMetrics<br/>count + time by route template"]
    metrics --> sec["SecurityHeaders"]
    sec --> hsts["HSTS<br/>production only"]
    hsts --> split{"path"}

    split -- "/swagger/*" --> swagger["Swagger UI"]
    split -- "/api/v1/*" --> csrf["RequireCSRF<br/>unsafe methods: X-CSRF-Token must equal csrf cookie"]

    csrf --> route{"route guard"}
    route -- "public<br/>/health, /auth/google/login,<br/>/auth/google/callback, /auth/logout" --> handler
    route -- "/me" --> session["RequireSession<br/>sid cookie → users_sessions"]
    route -- "/roles, /permissions" --> session2["RequireSession"] --> perm["RequirePermission<br/>feature + HTTP method → permission code"]

    session --> handler["Handler → Service → Repository"]
    perm --> handler
    handler --> pg[("PostgreSQL")]

    csrf -. "403 403001 no cookie / 403002 mismatch" .-> err["Error envelope<br/>{ error: { code, message } }"]
    session -. "401 401001 no cookie / 401002 expired" .-> err
    session2 -. "401 401001 / 401002" .-> err
    perm -. "403 403005 permission denied" .-> err
```

`RequirePermission` maps the method to an action (`GET` → read, `POST` → create, …),
builds the code for the route's feature, and checks it against the codes the user holds
through `users_roles` → `roles_permissions`.

---

## 3. Startup and shutdown

```mermaid
sequenceDiagram
    autonumber
    participant K as Kubernetes
    participant M as main.go
    participant C as config
    participant D as PostgreSQL
    participant G as Google OIDC

    K->>M: start container
    M->>C: Load() — embedded APP_ENV.yaml + environment
    Note over C: fails fast on a missing secret, empty<br/>allowed_origins, metrics_port = port,<br/>or a timeout ≤ 0
    M->>D: db.Connect — pool + ping
    M->>G: NewRouter → OIDC discovery
    Note over M: session cleanup goroutine starts,<br/>tied to the signal context
    M->>M: API and metrics listeners start
    K->>M: SIGTERM
    Note over M: both servers Shutdown within<br/>graceful_shutdown_in_second
    M->>D: pool.Close()
```

Every step before the listeners start exits the process on failure, so a pod with a bad
config or no database never reports ready.

---

## 4. From a tag to an image

```mermaid
flowchart LR
    tag["git push origin 0.3.1"] --> verify["verify<br/>lint, gofmt, vet, test -race"]
    verify -- "pass" --> build["build-and-push<br/>WIF auth → buildx linux/amd64"]
    build --> gar[("Artifact Registry<br/>league-service:0.3.1")]
    build --> notify["notify"]
    verify -- "fail: build skipped" --> notify
    notify -- "success" --> ok["Slack: green attachment<br/>image:tag@sha256:…"]
    notify -- "failure" --> ko["Slack: which job failed"]
    gar -. "rollout is outside this repo" .-> cluster["GKE"]
```

The tag is stamped into the binary and served back at `GET /api/v1/health` as `image`, so
what is running can be checked against what was pushed. Steps are in
[`workflow.md`](workflow.md) §10.
