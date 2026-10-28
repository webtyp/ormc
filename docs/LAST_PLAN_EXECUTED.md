---
PLAN: "feat!: generate a typed Item() from the fields marked Item:…; Row marks removed"
TAG: v0.1.26
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.

<!-- GATE PLAN: executed LOCALLY, second of three (model v0.2.8 → ormc → view). Needs webtyp.com/model v0.2.8. -->

# Plan — `webtyp.com/ormc`: write `Item()` for every listed model

## Why

`webtyp.com/model` v0.2.8 replaces the runtime `RowOf` walk with one type, `model.Item`, and a field mark that
names its slot (`Item: model.ItemLabel` / `ItemDescription` / `ItemBadge`). This generator now writes, for every
model with at least one mark, a typed method that reads the fields directly:

```go
func (m *PlanItem) Item() model.Item {
	return model.Item{ID: m.Id, Label: m.Object, Description: model.JoinItem(m.Before, m.After, m.Reason)}
}
```

No schema walk, no boxing, no conversion of a text field; a model without marks gets no `Item()` (and `view` will
not compile with it as a listed record — the link is checked by the compiler).

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go get webtyp.com/model@v0.2.8
```

Run the `go get` in the repo root **and** in `tests/` (separate module). If `model@v0.2.8` lacks `model.Item`,
`model.ItemSlot`, `model.JoinItem`, `model.ValidateItem` or `Field.Item`, stop and say so.

## Ecosystem search

`model.ValidateItem` (the one rule), `model.JoinItem` (the one separator), `model.Item` (the one type) → reuse.
`isInstant`, `goTypeUnix`, `goTypeNano`, `timeImportPath` (this repo, `storage.go`) → reuse for date fields.

## Design gate

1. **Prior art.** sqlc and protoc emit typed accessors instead of a reflective walk; Django/Rails declare the list
   projection once per model.
2. **Novice-name test.** The generated method is `Item()`, the type is `model.Item`, the mark is `Item: model.ItemLabel`.
3. **Complexity ledger.** One generated method per listed model; zero hand-written `Item()`; −1 runtime walk.
4. **Where it belongs.** Code generation from `model.Definition` is this repo's job.
5. **What it deletes.** `FieldInfo.Row`, `rowSlotOf`, `errUnknownRowSlot`, `validateRow` (renamed), the `"Row"` key.

## APIs used

| Symbol | Exact signature | Tag | File |
|---|---|---|---|
| `model.ItemSlot` | `type ItemSlot uint8`; `ItemNone` (0), `ItemLabel`, `ItemDescription`, `ItemBadge` | model v0.2.8 | `item.go` |
| `model.ValidateItem` | `func ValidateItem(fields []Field) error` (reads `Name`, `Item`, `Ref`, `Options`, `IsPK()`) | model v0.2.8 | `item.go` |
| `model.Field.Item` | `Item ItemSlot` | model v0.2.8 | `field.go` |

If a signature differs, stop and name it; never adapt by guessing.

## Stage 1 — parse `Item` instead of `Row` (`generator.go`, `parse_definition.go`)

- `FieldInfo`: replace `Row model.RowSlot` with `Item model.ItemSlot // list-item slot from Field.Item; ItemNone when absent`.
- `parseField`: the case `"Row"` becomes `"Item"`, calling `itemSlotOf`; rename `rowSlotOf` → `itemSlotOf` mapping
  `ItemLabel`, `ItemDescription`, `ItemBadge`, `ItemNone`; `errUnknownRowSlot` →
  `errUnknownItemSlot = "field %s: unknown Item slot %s (use model.ItemLabel, model.ItemDescription or model.ItemBadge)"`.
- A field still written `Row: …` is an error: add `case "Row": return FieldInfo{}, fmt.Err(fmt.Sprintf("field %s: Field.Row was replaced by Field.Item (model.ItemLabel, model.ItemDescription, model.ItemBadge)", fi.ColumnName))`.
- `validateRow` → `validateItem`: builds `model.Field{Name, Item: f.Item, DB: &model.FieldDB{PK: f.PK}}` plus the
  `Ref`/`Options` placeholders it builds today, and calls `model.ValidateItem`.

## Stage 2 — generate `Item()` (`generate.go`)

1. Imports: when any `info` has a field with `Item != model.ItemNone` whose `GoType != "string"`, add
   `kindImports["webtyp.com/fmt"] = ""` before the import block is written.
2. Right after the `Validate(action byte)` method is written for an `info`, call `writeItem(buf, info)`:

```go
// itemExpr is the Go expression turning field f of m into text for model.Item.
// A date field becomes seconds since 1970 (what the date/hour widgets read).
func itemExpr(f FieldInfo) string {
	switch {
	case f.GoType == "string":
		return "m." + f.Name
	case f.GoType == goTypeNano:
		return "fmt.Convert(int64(m." + f.Name + ")/1000000000).String()"
	case f.GoType == goTypeUnix:
		return "fmt.Convert(int64(m." + f.Name + ")).String()"
	default:
		return "fmt.Convert(m." + f.Name + ").String()"
	}
}

func writeItem(buf *fmt.Conv, info StructInfo) {
	var id, label, badge string
	var desc []string
	for _, f := range info.Fields {
		if f.PK && id == "" {
			id = itemExpr(f)
		}
		switch f.Item {
		case model.ItemLabel:
			label = itemExpr(f)
		case model.ItemDescription:
			desc = append(desc, itemExpr(f))
		case model.ItemBadge:
			badge = itemExpr(f)
		}
	}
	if label == "" {
		return // not a listed model: no Item()
	}
	parts := []string{"ID: " + id, "Label: " + label}
	switch len(desc) {
	case 0:
	case 1:
		parts = append(parts, "Description: "+desc[0])
	default:
		parts = append(parts, "Description: model.JoinItem("+strings.Join(desc, ", ")+")")
	}
	if badge != "" {
		parts = append(parts, "Badge: "+badge)
	}
	buf.Write(fmt.Sprintf("func (m *%s) Item() model.Item {\n\treturn model.Item{%s}\n}\n\n", info.Name, strings.Join(parts, ", ")))
}
```

   A field of a non-scalar kind (`FieldStruct`, `FieldStructSlice`, `FieldBlob`, `FieldIntSlice`) marked `Item` is
   a generation error: `field %s: only scalar fields can be shown in a list item`.
3. Check the generated output compiles: the existing `TestGenerate_E2E` style (generate into a temp module and
   build) must cover a model with marks.

## Stage 3 — tests (`generator_test.go`)

Rename the six `TestParseDefinition_Row*`/`…Options…` tests to the `Item` vocabulary (`Item: model.ItemLabel`, …)
and their expected messages (`two fields marked ItemLabel`, `need a field marked ItemLabel`, `unknown Item slot`).
Add:

| Test | Source | Expected in the generated file |
|---|---|---|
| `TestGenerate_ItemMethod` | `id` PK text, `object` ItemLabel, `before`/`after`/`reason` ItemDescription | `func (m *PlanItem) Item() model.Item {` and `model.JoinItem(m.Before, m.After, m.Reason)`; the file compiles |
| `TestGenerate_ItemOneDescriptionNoJoin` | one ItemDescription | `Description: m.Sku` (no `JoinItem`) |
| `TestGenerate_ItemDateBadgeIsSeconds` | `at` `model.NanoTime()` ItemBadge | `Badge: fmt.Convert(int64(m.At)/1000000000).String()` and `"webtyp.com/fmt"` imported |
| `TestGenerate_NoMarksNoItem` | no marks | no `Item()` in the output |
| `TestParseDefinition_RowKeyRejected` | `Row: model.RowTitle` | error containing `Field.Row was replaced by Field.Item` |

## Stage 4 — docs

`README.md`: the `Item` marks, the generated `Item()`, the date rule (seconds), the errors.

## Delivery

```
GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
cd tests && GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
```

Never edit a generated file by hand. Commit after every stage; never `git reset`/`restore`/`checkout --`/`stash`.

| Stage | File | Action |
|---|---|---|
| 1 | `generator.go`, `parse_definition.go`, `go.mod`, `go.sum`, `tests/go.mod`, `tests/go.sum` | parse `Item` |
| 2 | `generate.go` | write `Item()` |
| 3 | `generator_test.go` | behaviour |
| 4 | `README.md` | document |
