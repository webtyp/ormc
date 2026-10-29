---
PLAN: "fix: Models keeps a select's Options table and an instant's kind"
EXECUTOR: local
REVIEWER: none
---

# Plan — `webtyp.com/ormc`: `Models` loses nothing a diagram draws

## Why

`erd` draws a module's file from `ormc.Models`. The stub dropped `Field.Options` (a select filled from another
table: `device.zone_id` → `zone`) and turned `UnixTime`/`NanoTime` into a plain `Int`: the checked-in
diagram missed the link and the unit, while the app (real models) showed both.

## Ecosystem search

`FieldInfo` already carries `Options` (definition var) and `GoType` (`time.Unix`/`time.Nano`); `refTableName`
already maps a definition var to its table. Nothing new.

## Design gate

No exported symbol changes: `Models` keeps its signature and returns more of what the source declares.

| Stage | File | Action |
|---|---|---|
| 1 | `generate.go` (`newModelStub`) | `Options` → `&model.Definition{Name: refTableName(...)}`; `time.Unix`/`time.Nano` → `UnixTime()`/`NanoTime()` |
| 2 | `export_sql_test.go` | `TestModels_KeepsOptionsAndInstantKinds` |
| 3 | `README.md` | `Models` line |
