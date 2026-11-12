# VS Code tips

Three small setups: aligned struct tags, debug logs you cannot forget to delete, and
foldable regions.

Replace `vincent` throughout with your own name or handle. It has to be a string nobody
else in the repo would type — that uniqueness is the whole trick.

---

## 1. Align struct tags on save

`gofmt` aligns field names, types and the opening backtick, but **not the contents inside
the tag**. On a struct with two tags you get this:

```go
// gofmt
ID    string `json:"id" db:"id"`
Email string `json:"email" db:"email"`

// formattag
ID    string `json:"id"    db:"id"`
Email string `json:"email" db:"email"`
```

Setup:

```bash
go install github.com/momaek/formattag@v0.0.8
```

Install [Run on Save](https://marketplace.visualstudio.com/items?itemName=emeraldwalk.RunOnSave),
then add to `settings.json`:

```json
"emeraldwalk.runonsave": {
  "commands": [
    {
      "match": "\\.go$",
      "cmd": "formattag -file ${file}"
    }
  ]
}
```

If the binary isn't on your `PATH`, use the full path instead — `~/go/bin/formattag` on
macOS and Linux.

Its output stays `gofmt`-clean, so this won't fight CI (which runs `test -z "$(gofmt -l .)"`).
Note it's a no-op on single-tag structs, which is most of this repo today — it earns its
place once a struct carries both `json:` and `db:`.

---

## 2. Debug logs and TODOs you can always find

Every snippet below stamps `vincent` into the line it writes. One search finds all of
them, and only yours.

`Cmd/Ctrl+Shift+P` → **Configure User Snippets** → `go.json`:

```json
{
  "todo": {
    "prefix": "todo",
    "body": ["// TODO vincent $1\n$0"],
    "description": "TODO comment"
  },
  "log": {
    "prefix": "xxx",
    "body": ["fmt.Printf(\"\\nvincent %#v\\n\", $1)\n$0"],
    "description": "debug log"
  },
  "dump": {
    "prefix": "dump",
    "body": [
      "b, _ := json.MarshalIndent(${1:v}, \"\\t\", \"\\t\");fmt.Printf(\"vincent %s\\n\", string(b))\n$0"
    ],
    "description": "dump a struct as JSON"
  },
  "dumphttp": {
    "prefix": "dumphttp",
    "body": [
      "d, _ := httputil.DumpRequest(${1:r}, true);fmt.Printf(\"vincent %s\\n\", string(d))\n$0"
    ],
    "description": "dump an http request"
  }
}
```

Type the prefix in a `.go` file and accept the completion. `dump` needs `encoding/json`
and `dumphttp` needs `net/http/httputil` — `gopls` adds both on save.

**Before committing**, clear the debug lines:

```bash
grep -rn "vincent" --include="*.go" . | grep -v "TODO vincent"
```

Nothing in CI catches a stray `fmt.Printf` — not `gofmt`, not `go vet`, not
`golangci-lint`. This grep is the only gate, so run it. The `grep -v` keeps your
intentional TODOs out of the way; drop it to list those instead:

```bash
grep -rn "TODO vincent" --include="*.go" .
cmd/server/main.go:68:  // TODO vincent: take these from cfg.Server once those fields are wired.
```
