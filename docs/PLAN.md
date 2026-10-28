---
PLAN: "feat: Models(root) — the scanned models, in file order"
TAG: v0.1.27
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 12065715035981192446
PR: https://github.com/webtyp/ormc/pull/4
REVIEWED_COMMIT: e8e772e8bbbd55d9a7af1f8f65428d0ca5fa9fbc
---

> Read AGENTS.md at the repo root before any change.

# Plan — `webtyp.com/ormc`: `Models(root)`

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
```

## Why

`webtyp.com/erd` (database diagram) and the `webtyp/app` dev console need the models a repo declares, without
SQL. Today the only way to get them is `ExportSQL(root, exporter)`, which hands them to a DDL exporter and
iterates a map (random order). `Models(root)` returns them in file order; `ExportSQL` becomes `Models` + the
exporter, so there is one path.

## Ecosystem search

`ormc.collectAllStructs` (unexported) already scans; `newModelStub` already builds a `model.Model` with
`SchemaExt()`. Extend `ormc`: nothing new elsewhere.

## Design gate

1. **Prior art:** `go/packages.Load` (returns packages, caller filters), sqlc `Catalog`, GORM `Migrator().GetTables`
   — all expose the scanned schema as data before any rendering; ormc only exposed it through a renderer.
2. **Name:** `g.Models(root)` — «the models of root». Same word as `model.Model`.
3. **Ledger:** concepts +1 (`Models`) / −0; ways to get the models +0 (ExportSQL now calls it).
4. **Belongs:** ormc owns the scan; the diagram and the console only consume it.
5. **Deletes:** the duplicated scan loop inside `ExportSQL`.

## Stage 1 — `generate.go`

Replace the body of `ExportSQL` and add `Models` right above it:

```go
// Models scans root for model.go/models.go and returns one model.Model per database struct
// (NoDB structs skipped), in the order the files and declarations were read — the same root
// always gives the same order. A model's foreign keys are in its SchemaExt(). It returns
// ErrNoModelsFound when there is none.
func (g *Generator) Models(root string) ([]model.Model, error) {
	g.rootDir = root
	all, order, _, err := g.collectAllStructs()
	if err != nil {
		return nil, err
	}
	var models []model.Model
	for _, name := range order {
		info := all[name]
		if info.NoDB {
			continue
		}
		models = append(models, newModelStub(info))
	}
	if len(models) == 0 {
		return nil, ErrNoModelsFound
	}
	return models, nil
}

// ExportSQL scans the directory for models and returns the full DDL.
// Requires an injected Exporter (e.g. from sqlt or postgres).
func (g *Generator) ExportSQL(root string, exporter Exporter) (string, error) {
	models, err := g.Models(root)
	if err != nil {
		return "", err
	}
	return exporter.ExportDDL(models)
}
```

Keep the existing doc comment of `ExportSQL` if it says more than the one above.

## Stage 2 — tests (`export_sql_test.go`, package `ormc`, next to the existing test)

```go
func TestModels_SameOrderEveryTimeAndKeepsForeignKeys(t *testing.T) {
	first, err := New().Models("tests")
	if err != nil {
		t.Fatal(err)
	}
	second, _ := New().Models("tests")
	if len(first) == 0 || len(first) != len(second) {
		t.Fatalf("len = %d and %d, want equal and > 0", len(first), len(second))
	}
	for i := range first {
		if first[i].ModelName() != second[i].ModelName() {
			t.Fatalf("order differs at %d: %s vs %s", i, first[i].ModelName(), second[i].ModelName())
		}
	}
	for _, m := range first {
		if m.ModelName() != "order" {
			continue
		}
		ext := m.(interface{ SchemaExt() []model.FieldExt }).SchemaExt()
		if len(ext) != 1 || ext[0].Ref != "user" {
			t.Fatalf("order SchemaExt = %+v, want one FK to user", ext)
		}
		return
	}
	t.Fatal("model order not found in tests/")
}

func TestModels_NoModels_ReturnsErrNoModelsFound(t *testing.T) {
	if _, err := New().Models(t.TempDir()); !errors.Is(err, ErrNoModelsFound) {
		t.Fatalf("err = %v, want ErrNoModelsFound", err)
	}
}
```

`tests/models.go` declares `OrderModel` with `user_id` → `Ref: &UserModel`; if the stub stores the ref under a
different table name than `"user"`, stop and say so in the PR (do not change the fixture).

## Stage 3 — `README.md`

In the API section, add one line: `Models(root) ([]model.Model, error)` — the database models of root in file
order, foreign keys in `SchemaExt()`; `ErrNoModelsFound` when none. Used by `webtyp/erd`.

## Delivery

- `gotest` green. Environment: `GOPRIVATE=webtyp.com/*,github.com/veltylabs/*`, `GOFLAGS=-mod=mod`. A PR that does
  not compile, or whose tests could not run, is a failed delivery.
- `grep -n "for _, info := range all" generate.go` → empty (the map loop is gone).
- `git diff --stat main` lists exactly: `generate.go`, `export_sql_test.go`, `README.md`.
- Never pause to ask permission or report progress: when the check passes, commit, push and open the PR. Stuck 30
  minutes → open the PR listing what fails. Commit after every stage; never `git reset`, `git restore`,
  `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `generate.go` | `Models`; `ExportSQL` calls it |
| 2 | `export_sql_test.go` | order is stable, FK kept, no models → error |
| 3 | `README.md` | one line |
