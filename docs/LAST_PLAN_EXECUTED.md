---
PLAN: "fix: stop generating Schema()/Pointers() stubs on list types"
EXECUTOR: jules
REVIEWER: none
---

> This plan is dispatched via the CodeJob workflow. See skill: agents-workflow.
>
> **Phase B (GATE)** of
> [`LIST_CONTRACT_MASTER_PLAN.md`](https://github.com/webtyp/docs/blob/main/LIST_CONTRACT_MASTER_PLAN.md).
> The 12 consumer repos (phase C) cannot start before this ships a tag.
>
> **Depends on phase A** (`webtyp.com/model` narrowing `FielderSlice`): as the
> first line of work, `go get webtyp.com/model@latest`. Never add a `replace`,
> never invent a version.

# Plan — `webtyp.com/ormc`: stop emitting two methods that answer a question a list cannot have

## 0. Context (verified against the repo — do not re-diagnose)

`generate.go` emits five methods for every list type. The first two are the
defect:

```go
// generate.go, lines 306-310
buf.Write(fmt.Sprintf("func (s *%sList) Schema() []model.Field { return nil }\n", info.Name))
buf.Write(fmt.Sprintf("func (s *%sList) Pointers() []any     { return nil }\n", info.Name))
buf.Write(fmt.Sprintf("func (s *%sList) Len() int             { return len(*s) }\n", info.Name))
buf.Write(fmt.Sprintf("func (s *%sList) At(i int) model.Fielder { return (*s)[i] }\n", info.Name))
buf.Write(fmt.Sprintf("func (s *%sList) Append() model.Fielder  { v := &%s{}; *s = append(*s, v); return v }\n", info.Name, info.Name))
```

They existed only because `model.FielderSlice` embedded `model.Fielder`, so a
generated list had to answer "what are your columns?" — a question a sequence of
rows cannot have. Phase A removed that embedding, so the two stubs are now dead
weight that nothing requires.

They are worse than dead: because a list carries `Schema()`/`Pointers()`, it
satisfies `model.Fielder`, so `Accepts(&UserList{})` compiles and
`mcp/tool_schema.go` publishes the tool advertising **no arguments** — silently.
Deleting them is what makes that state unrepresentable.

**This is not a size optimization.** It was measured: ~27 bytes per list type,
0,02 % of a real WASM client. Do not justify or scope this change by binary
size — see §2 of the master plan.

**Anti-footgun.** Only the first two lines go. `Len`, `At` and `Append` are the
whole contract now and must keep being emitted exactly as they are. Likewise
`EncodeFields`/`DecodeFields` no-ops on lists stay: `json.Encode` takes a
`model.Encodable`, so removing them would break every call that serializes a
list. That alternative was measured and rejected — do not extend this change
into it.

## Quality rules

```
RULE: every repeated string is a named constant; string literals forbidden in logic.
RULE: the generator's output must stay gofmt-stable — regenerating twice yields
      a byte-identical file.
RULE: do not change any other emitted method, ordering, or spacing.
```

## Stage 1 — delete the two emissions

**File:** `generate.go`.

Delete exactly these two lines (306 and 307 at the time of writing; match on
content, not line number):

```go
buf.Write(fmt.Sprintf("func (s *%sList) Schema() []model.Field { return nil }\n", info.Name))
buf.Write(fmt.Sprintf("func (s *%sList) Pointers() []any     { return nil }\n", info.Name))
```

Leave the three that follow untouched. Add a short comment above the remaining
block stating why the list has no `Schema`/`Pointers`:

```go
// A list exposes only traversal: it is a sequence of rows and has no columns of
// its own, so model.FielderSlice does not embed model.Fielder. The schema comes
// from the element, via At/Append.
```

## Stage 2 — regenerate this repo's own fixtures

**File:** `tests/models_orm.go` (14 list types).

This repo is also a consumer of its own generator. Regenerate it with the
rebuilt `ormc` so the golden output matches what the generator now emits, then
confirm the package still builds and its tests pass.

Do not hand-edit the generated file: run the generator.

**`tests/` is a SEPARATE Go module.** It has its own `tests/go.mod`, and its
header says why:

```
// Separate test module: isolates codegen fixture deps (orm runtime for
// generated models) so the root webtyp.com/ormc module stays
// fmt + model + modfind only.
```

So:

1. Run `go get webtyp.com/model@latest` **inside `tests/`** as well — it
   resolves its dependencies independently of the root module.
2. Run the generator from where the definitions live (`tests/`), not only from
   the repo root.
3. The root `go.mod` must keep requiring **only** `fmt`, `model` and `modfind`.
   If `webtyp.com/orm` appears there after your work, you regenerated from the
   wrong directory — revert and redo it. This is the one invariant this repo is
   organised around; breaking it defeats the split.

## Stage 3 — the generator test pins the absence

**File:** `generator_test.go`.

The existing suite has cases like `TestGenerate_OmitEmpty` that assert on the
produced source. Add `TestGenerate_ListHasNoSchema` in the same style:

1. Generate from a minimal definition with a PK and one text field.
2. Assert the list keeps its traversal methods — verbatim substrings:
   - `` `func (s *ModelList) Len() int` ``
   - `` `func (s *ModelList) At(i int) model.Fielder` ``
   - `` `func (s *ModelList) Append() model.Fielder` ``
3. Assert the two stubs are **absent**. Verbatim failure message:
   `` `generated list must not declare Schema(): a list has no columns of its own` ``
   and the same shape for `Pointers()`.
   Match on `` `func (s *ModelList) Schema()` `` and
   `` `func (s *ModelList) Pointers()` `` — do NOT match on the bare words
   `Schema(` or `Pointers(`, which legitimately appear on the element type in
   the same file.

## Acceptance criteria

1. `go build ./...`, `go vet ./...`, `go test ./...` green.
2. `grep -rn "List) Schema()" --include='*.go' .` → empty, including
   `tests/models_orm.go`.
3. `grep -rn "List) Pointers()" --include='*.go' .` → empty.
4. `grep -rn "List) Len()\|List) At(\|List) Append()" tests/models_orm.go` →
   14 of each: the traversal contract survived intact.
5. Running the generator twice produces a byte-identical `tests/models_orm.go`.
6. Both `go.mod` and `tests/go.mod` require the phase A tag of
   `webtyp.com/model`; no `replace` in either.
7. The root `go.mod` still requires only `fmt`, `model` and `modfind` — the
   module split survived.

## Out of scope

- Regenerating the other 11 repos — phase C, one plan each.
- The hand-written lists in `mcp`, `svg` and `view` — phase C; this generator
  never emitted them.
- Removing the `EncodeFields`/`DecodeFields` no-ops from lists — measured and
  rejected, see the anti-footgun above.

| Stage | Files | Action |
|---|---|---|
| 1 | `generate.go` | delete the two `Schema`/`Pointers` emissions; comment why |
| 2 | `tests/models_orm.go` | regenerate with the rebuilt generator |
| 3 | `generator_test.go` | `TestGenerate_ListHasNoSchema` pins traversal-only |
