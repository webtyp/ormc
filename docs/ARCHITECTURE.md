# ormc — Architecture

> WHAT ormc is and WHY it works the way it does. The debate behind the
> storage-resolution decision lives in [DESIGN.md](DESIGN.md).

## What ormc is

`webtyp.com/ormc` is the **code generator** of the webtyp data
stack — nothing else. It reads hand-written `model.Definition` literals and
emits the concrete Go artifacts the runtime consumes: the plain struct, the
`Fielder` methods (`Schema()`, `Pointers()`, codec), `Validate()`, typed
query-field helpers (`User_.Email`), and DDL FK metadata (`SchemaExt()`).

It was split out of `webtyp.com/orm` (2026-07-10). The split
boundaries:

| Repo | Role | ormc's relationship |
|---|---|---|
| `webtyp/model` | field/kind contract (`Definition`, `Field`, `Kind`) | ormc's INPUT contract |
| `webtyp/ormc` | this repo: generator + `cmd/ormc` | — |
| `webtyp/ddlc` | DDL migration tool (`tui`) | not a dependency of ormc anymore |
| `webtyp/orm` | query/scan/sync runtime | imported by generated read helpers |
| `webtyp/sqlmcp` | MCP provider over the stack | consumer, not a dependency |

ormc runs in two modes: standalone CLI (`cmd/ormc`) and **live watcher**
inside the dev tool (`webtyp/app` feeds file events; see
[SYNC_DESIGN.md](SYNC_DESIGN.md) and
[diagrams/DB_SYNC.md](diagrams/DB_SYNC.md)). The watcher mode is a hard
architectural constraint: generation runs on every file save, often against
code that does not compile yet.

## Input contract (authoring)

A model is a `model.Definition` composite literal. Each field declares its
kind in the single `Type:` slot as a **constructor expression**:

```go
var UserModel = model.Definition{Fields: []model.Field{
    {Name: "email",    Type: input.Email(), NotNull: true}, // form kind (validates + renders)
    {Name: "status",   Type: model.Text()},                  // base kind (validates only)
    {Name: "address",  Type: model.Struct(&AddressModel)},   // composition: ref IN the constructor
    {Name: "staff_id", Type: model.Int(), Ref: &StaffModel}, // scalar FK: Ref's ONLY meaning
}}
```

Contract rules (all violations are **hard generation errors** — ormc never
guesses):

- `Type:` is mandatory on every field (fail-closed; the compile-time guard
  for `model`'s always-on validation).
- `Widget:` no longer exists (Kind unification deleted it; the actionable
  error tells the author to move the constructor into `Type:`).
- Composition refs travel inside `Struct(ref)`/`StructSlice(ref)`;
  `Field.Ref` means scalar foreign key and nothing else. `Ref:` next to a
  composition constructor is a contradiction → error.
- A project-custom kind must live in its **own package**, separate from the
  Definitions that use it (see storage resolution below for why).
- Constructor arguments must be self-contained literals — they may not
  reference identifiers of the scanned package.

## Generation mechanism: AST over the user's source, always

ormc parses the Definition literal with `go/ast`. It **never compiles or
executes the user's package**. Rationale (settled in the model repo,
`docs/ARCHITECTURE.md` §9,
<https://github.com/webtyp/model/blob/main/docs/ARCHITECTURE.md>):

1. The watcher regenerates on every save; mid-edit code frequently does not
   compile — exactly when regeneration is needed.
2. Chicken-and-egg: the generated file is what makes the package compile.
3. AST needs no build and creates no import cycle between generator and
   model.

## Storage resolution

Generated struct fields need each kind's `FieldType` at generation time,
but the new contract keeps storage behind the `Kind.Storage()` method — a
runtime value, no longer a constant in the user's source. Resolution order:

1. **Builtin table** for the closed set of `model.*` constructors
   (`model.Text` → `FieldText`, …; the two composition constructors derive
   their Go type from the captured ref argument).
2. **Cached dependency probe** for every other constructor: ormc generates
   a throwaway `main` that imports the kinds' packages, executes each
   captured constructor, and reads the REAL `Storage()` value. Key
   distinction: the no-compile rule above protects the **user's package**;
   kind packages are ordinary, always-compilable **dependencies** of the
   scanned module. Results are cached by (module root, `go.mod` hash,
   constructor set) — watcher saves with no new constructors never re-run
   the probe. There is no annotation, registry, or marker to author, and no
   second copy of the storage fact that could drift: the probe executes the
   single source of truth. Alternatives considered and rejected:
   [DESIGN.md](DESIGN.md).
3. **Hard error** otherwise — probe failures surface the compiler output
   verbatim; a probed kind resolving to struct/structslice storage is
   rejected (custom composition kinds are unsupported by design).

## Generated-code contract

Generated files (`*_orm.go`) import:

- `webtyp/model` — always (schema types).
- `webtyp/orm` — when the model has DB role (query helpers `*orm.QB`).
- The kind constructors' packages (e.g. `webtyp/input`) — `Schema()`
  re-emits every `Type:` constructor expression **verbatim**; ormc passes
  kinds through without understanding them.

The typed-field helpers (`User_.Email` used by `.Where(...).Eq(...)`) are
part of the stable contract and are always generated. On why the generated
volume costs nothing in WASM binaries:
[WHY_GENERATED_CODE_IS_FREE.md](WHY_GENERATED_CODE_IS_FREE.md).

Flow diagram: [diagrams/ORMC_FLOW.md](diagrams/ORMC_FLOW.md).

## Failure doctrine

Every ambiguity is a loud generation error naming the field/constructor and
the fix. ormc never silently defaults a storage type, never skips a field
it cannot parse, and never rewrites user source beyond the generated
`*_orm.go` file.
