# Rules: naming

Applies to every Go file. A name says what the thing is or does without the reader
opening its package or its body. Clarity beats brevity.

This deliberately departs from Effective Go's "avoid stutter" advice:
`permissions.ListPermissions` is preferred over `permissions.List`. A full name greps,
reads unambiguously in a stack trace or log line, and matches the repository style
already used in `auth` (`GetUserByID`, `DeleteSession`).

---

## 1. Functions and methods: verb + entity

Every handler, service and repository method names its action and its entity.

### Avoid
```go
func (h *Handler) List(w http.ResponseWriter, r *http.Request)
func (s *service) All(ctx context.Context) ([]Role, error)
func (r *repository) ByID(ctx context.Context, id string) (*Role, error)
```

### Prefer
```go
func (h *Handler) ListRoles(w http.ResponseWriter, r *http.Request)
func (s *service) ListRoles(ctx context.Context) ([]Role, error)
func (r *repository) GetRoleByID(ctx context.Context, id string) (*Role, error)
```

Verbs, used consistently across layers: `List…` (many), `Get…` / `Get…ByID` (one),
`Create…`, `Update…`, `Delete…`. A function that returns an `error` value is named
for the error (`superAdminProtectedError`, `InvalidJSONBodyError`).

---

## 2. Types and constants carry the entity

Package-level names are read far from their declaration.

### Avoid
```go
type Response struct{ … }
const columns = "id, code, name"
const listLimit = 200
var ErrNotFound = errors.New("roles: not found")
```

### Prefer
```go
type RoleResponse struct{ … }
const roleColumns = "id, code, name"
const roleListLimit = 200
var ErrRoleNotFound = errors.New("roles: not found")
```

---

## 3. Parameters and variables: words, not letters

### Avoid
```go
func (r *repository) CreateRole(ctx context.Context, p CreateRoleServiceParams) (*Role, error)
func (r *repository) ReplaceSession(ctx context.Context, s Session) error
var u User
```

### Prefer
```go
func (r *repository) CreateRole(ctx context.Context, params CreateRoleServiceParams) (*Role, error)
func (r *repository) ReplaceSession(ctx context.Context, session Session) error
var user User
```

---

## 4. What stays short

Go idiom wins for these: everyone reads them the same way, and a longer form adds
nothing.

| Kept as is | Why |
|---|---|
| `ctx`, `err`, `w`, `r`, `id`, `i` | universal Go names |
| receivers: `h *Handler`, `s *service`, `r *repository`, `m *middleware` | one or two letters by convention |
| `New`, `NewService`, `NewRepo`, `NewHandler` | constructors; the package names the type |
| `Load`, `Connect`, `Seal`, `Open` | the package already says what is loaded or sealed |

---

## 5. Acronyms keep one case

`HTTPMethodToPermissionAction`, `GetUserByID`, `ParsePathUUID` — never `Http…`, `Id`,
`Uuid`.
