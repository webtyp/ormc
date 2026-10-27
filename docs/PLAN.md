---
PLAN: "feat: read Field.Row and Field.Options; reject an invalid list row at generation time"
TAG: v0.1.25
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 14563173585899449152
---

> Read AGENTS.md at the repo root before any change.

# Plan — `webtyp.com/ormc`: list-row declarations fail at generation, not at runtime

## Why

`webtyp.com/model` v0.2.7 lets a field say where it shows in a list row (`Row: model.RowTitle` / `RowDetail` /
`RowBadge`) and which list fills its choices without a foreign key (`Options: &ZoneModel`). The rule is
`model.ValidateRow` (at most one title and one badge, a title whenever anything is in the row, never `Ref` and
`Options` together). `ormc` reads every `model.Definition` when the developer saves a file, so a broken row must stop
generation there, with the model's name — not show up later as an empty row in the browser.

Nothing new is generated: `Schema()` already returns the hand-written `XModel.Fields`, so `Row` and `Options` reach
runtime by themselves.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/model@v0.2.7
```

Run the `go get` in the repo root **and** in `tests/` (a separate module: `tests/go.mod`). `model@v0.2.7` is the first
tag with `Field.Row`, `Field.Options`, `model.RowSlot` and `model.ValidateRow`; if it lacks them, stop and say so in the PR.

## Ecosystem search

- `model.ValidateRow(fields []model.Field) error` (model v0.2.7, `row.go`) is THE rule → **call it**; never restate it here.
- `parseRef(expr)` (`parse_definition.go:297`) already turns `&ZoneModel` into `ZoneModel` → reuse for `Options`.
- `StructInfo.asFields()` (`generator.go:325`) builds `[]model.Field` for another purpose (DDL probing) → not reused:
  it drops `Ref`; Stage 2 builds the slice the rule needs.

## Design gate

1. **Prior art.** Code generators validate declarations before emitting: `sqlc` rejects a query whose columns do not
   exist, `protoc` rejects a duplicate field number, Prisma's `prisma generate` rejects two `@id` fields.
2. **Novice-name test.** No new exported name.
3. **Complexity ledger.** Concepts +0; places a row rule lives: 1 (`model`), called from 2 moments (generation here,
   startup in renderers).
4. **Where it belongs.** The rule in `model`; the moment (generation) here.
5. **What it deletes.** Nothing; prevents broken rows from reaching runtime.

## APIs used

| Symbol | Exact signature | Tag | File |
|---|---|---|---|
| `model.RowSlot` | `type RowSlot uint8`; constants `RowNone` (0), `RowTitle`, `RowDetail`, `RowBadge` | model v0.2.7 | `row.go` |
| `model.ValidateRow` | `func ValidateRow(fields []Field) error` — reads `Field.Name`, `Field.Row`, `Field.Ref`, `Field.Options` | model v0.2.7 | `row.go` |
| `model.Field.Row` | `Row RowSlot` | model v0.2.7 | `field.go` |
| `model.Field.Options` | `Options *Definition` | model v0.2.7 | `field.go` |

If a signature in the published tag differs from this table, stop and name the symbol in the PR; never adapt it by guessing.

## Stage 1 — parse the two keys (`generator.go`, `parse_definition.go`)

1. `FieldInfo` (`generator.go:28`): add after `OnDelete`:

```go
	Row     model.RowSlot // list-row slot from Field.Row; RowNone when absent
	Options string        // Definition var name from Field.Options (e.g. "ZoneModel"); "" when absent
```

2. `parseField` (`parse_definition.go`), add two cases to the `switch key`:

```go
		case "Row":
			slot, ok := rowSlotOf(exprToString(kv.Value))
			if !ok {
				return FieldInfo{}, fmt.Err(fmt.Sprintf(errUnknownRowSlot, fi.ColumnName, exprToString(kv.Value)))
			}
			fi.Row = slot
		case "Options":
			fi.Options = parseRef(kv.Value)
```

3. Add to `parse_definition.go`:

```go
const errUnknownRowSlot = "field %s: unknown Row slot %s (use model.RowTitle, model.RowDetail or model.RowBadge)"

// rowSlotOf maps the selector written in a Definition (model.RowTitle, or RowTitle
// with a dot-import) to its slot.
func rowSlotOf(expr string) (model.RowSlot, bool) {
	name := expr
	if i := strings.LastIndex(expr, "."); i >= 0 {
		name = expr[i+1:]
	}
	switch name {
	case "RowTitle":
		return model.RowTitle, true
	case "RowDetail":
		return model.RowDetail, true
	case "RowBadge":
		return model.RowBadge, true
	case "RowNone":
		return model.RowNone, true
	}
	return model.RowNone, false
}
```

Use the same `fmt` package `parse_definition.go` already imports for `fmt.Err`/`fmt.Sprintf`.

## Stage 2 — validate each Definition (`parse_definition.go`, `parseDefinition`)

Right after the `switch key` loop over `cl.Elts` (after `info.Fields = fields` is set) and before
`if info.ModelName == ""`, add:

```go
	if err := validateRow(varName, info.Fields); err != nil {
		return nil, err
	}
```

and the helper:

```go
// validateRow hands model.ValidateRow the facts it reads (name, row slot, and
// whether Ref/Options are set) and names the Definition in the error.
func validateRow(varName string, fields []FieldInfo) error {
	mf := make([]model.Field, len(fields))
	for i, f := range fields {
		mf[i] = model.Field{Name: f.ColumnName, Row: f.Row}
		if f.Ref != "" && f.Type != model.FieldStruct && f.Type != model.FieldStructSlice {
			mf[i].Ref = &model.Definition{Name: f.Ref}
		}
		if f.Options != "" {
			mf[i].Options = &model.Definition{Name: f.Options}
		}
	}
	if err := model.ValidateRow(mf); err != nil {
		return fmt.Err(varName + ": " + err.Error())
	}
	return nil
}
```

(Composition fields carry `Ref` from their constructor argument and are not foreign keys: they are excluded above.)

## Stage 3 — tests (`generator_test.go`, root package, same harness as `TestParseDefinition_Basic`)

Each test writes a source with `writeTemp` and calls `g.parseDefinitionsInFile(path)`.

| Test | `Fields` of the source | Expected |
|---|---|---|
| `TestParseDefinition_RowSlots` | `id` PK; `name` `Row: model.RowTitle`; `sku` `Row: model.RowDetail` | no error; `Fields[1].Row == model.RowTitle`, `Fields[2].Row == model.RowDetail` |
| `TestParseDefinition_Options` | `zone_id` `Type: model.Text(), Options: &ZoneModel` (and a `ZoneModel` with `id`, `name` `Row: model.RowTitle` in the same source) | no error; `Fields[i].Options == "ZoneModel"` |
| `TestParseDefinition_TwoRowTitlesFails` | two fields `Row: model.RowTitle` in `DeviceModel` | error containing `DeviceModel: ` and `two fields marked RowTitle` |
| `TestParseDefinition_DetailWithoutTitleFails` | one `RowDetail`, no title | error containing `need a field marked RowTitle` |
| `TestParseDefinition_RefAndOptionsFails` | `zone_id` with `Ref: &ZoneModel` and `Options: &ZoneModel` | error containing `both Ref and Options: zone_id` |
| `TestParseDefinition_UnknownRowSlotFails` | `Row: model.RowFooter` | error containing `unknown Row slot` |

All existing tests (`TestGenerate_E2E`, `TestGenerate_FK_SchemaExt`, …) stay green unchanged.

## Stage 4 — docs

`README.md` (and `docs/` architecture file if present): one paragraph — `ormc` rejects an invalid list row
(`model.ValidateRow`) when it generates; `Row` and `Options` need no generated code because `Schema()` returns the
Definition's fields.

## Delivery

```
GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
cd tests && GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
```

- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never edit a generated `*_orm.go` file (including `tests/models_orm.go`); if one must change, regenerate it with
  this repo's own generator.
- Never pause to ask permission or report progress: when the check passes, commit, push and open the PR. Stuck 30
  minutes → open the PR listing what fails. Commit after every stage; never `git reset`, `git restore`,
  `git checkout -- <file>` or `git stash` on your own work.

| Stage | File | Action |
|---|---|---|
| 1 | `generator.go`, `parse_definition.go`, `go.mod`, `go.sum`, `tests/go.mod`, `tests/go.sum` | parse `Row`, `Options` |
| 2 | `parse_definition.go` | `model.ValidateRow` per Definition |
| 3 | `generator_test.go` | 6 tests |
| 4 | `README.md` | document |
