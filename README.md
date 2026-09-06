# ormc
<img src="docs/img/badges.svg">

Go code generator for `webtyp/model` definitions: reads hand-written
`model.Definition` literals via AST and emits the concrete struct, schema,
codec, validation, and typed query helpers the runtime consumes.

## Quick-start

Install the CLI:

```bash
go install webtyp.com/ormc/cmd/ormc@latest
```

Author a `model.go` (or `models.go`) with one or more `model.Definition`
literals. Each field declares its kind in the single `Type:` slot as a
constructor expression — never a bare enum, never a struct tag:

```go
package myapp

import (
	"webtyp.com/model"
	"webtyp.com/input"
)

var AddressModel = model.Definition{
	Name: "address",
	Fields: model.Fields{
		{Name: "street", Type: model.Text()},
		{Name: "city", Type: model.Text()},
	},
}

var UserModel = model.Definition{
	Name: "user",
	Fields: model.Fields{
		{Name: "id", Type: model.Int(), DB: &model.FieldDB{PK: true, AutoInc: true}},
		{Name: "email", Type: input.Email(), NotNull: true},   // form kind: validates + renders
		{Name: "status", Type: model.Text()},                   // base kind: validates only
		{Name: "address", Type: model.Struct(&AddressModel)},   // composition: ref IN the constructor
		{Name: "manager_id", Type: model.Int(), Ref: &UserModel}, // scalar FK: Ref's ONLY meaning
	},
}
```

Run the generator from the module root:

```bash
ormc
```

It walks the module for `model.go`/`models.go` files and writes one
`<file>_orm.go` per source file, containing the plain struct, the
`Fielder` methods (`Schema()`, `Pointers()`, encode/decode), `Validate()`,
typed query-field helpers (`User_.Email`), and `SchemaExt()` for any scalar
FKs. `model.*` kinds resolve via a builtin table; every other kind (form
kinds, project-custom kinds) resolves by executing its real `Storage()`
through a cached, generation-time dependency probe — see
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) for the resolution order and
failure modes.

Inside [webtyp/app](https://github.com/webtyp/app)'s dev console, ormc
runs as a live TUI handler instead of the standalone CLI: `New()` returns a
`*Generator` implementing the handler contract (`Name()`,
`SupportedExtensions()`, `NewFileEvent(...)`), so the tool regenerates the
affected `_orm.go` on every save and, once `SetSyncer` is called, syncs the
DB schema. See [docs/SYNC_DESIGN.md](docs/SYNC_DESIGN.md) and
[docs/diagrams/DB_SYNC.md](docs/diagrams/DB_SYNC.md) for the watcher/sync
flow.

## Documentation

- [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md) — what ormc is post-split,
  input contract (`Type:` constructor expressions), AST-only mechanism,
  storage resolution order, generated-code contract.
- [docs/DESIGN.md](docs/DESIGN.md) — decision record: why storage resolves
  via the cached dependency probe (directive and marker alternatives
  rejected).
- [docs/SYNC_DESIGN.md](docs/SYNC_DESIGN.md) — rationale and trade-offs of
  tool-driven dev schema sync.
- [docs/WHY_GENERATED_CODE_IS_FREE.md](docs/WHY_GENERATED_CODE_IS_FREE.md) —
  why the generated volume costs zero bytes in WASM binaries.
- [docs/WHY_PACKAGE_LEVEL_SCHEMA.md](docs/WHY_PACKAGE_LEVEL_SCHEMA.md) —
  why generated `Schema()` returns a package-level variable (zero alloc).
- [docs/diagrams/ORMC_FLOW.md](docs/diagrams/ORMC_FLOW.md) — Definition →
  generated code flow, error table, runtime validation.
- [docs/diagrams/DB_SYNC.md](docs/diagrams/DB_SYNC.md) — dev database
  schema-sync flow (tool-driven).
