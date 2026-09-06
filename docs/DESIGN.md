# DESIGN — storage resolution for non-`model` kinds (decision record)

> Companion to [ARCHITECTURE.md](ARCHITECTURE.md) (which states only the
> outcome). This file records the problem, the candidates, and why the
> dependency probe won. Settled 2026-07-10.

## The problem

Before Kind unification, storage was a constant written literally in the
user's source (`Type: model.FieldText`) — the AST parser simply read it.
Phase A collapsed `Type` (enum) + `Widget` (constructor) into one typed slot
(`Type: input.Email()`) to kill the expressible contradiction and the
fail-open default. Side effect: **the storage fact moved from source text to
the return value of `Kind.Storage()`** — invisible to a parser.

ormc cannot call `Storage()` the obvious way because it never compiles or
executes the user's package (watcher on every save, mid-edit broken code,
chicken-and-egg — see model repo `docs/ARCHITECTURE.md` §9).

## Candidates

### 1. `//ormc:storage <type>` comment directive — REJECTED

A comment above each kind constructor, read from the package source.

- Violates the harness doctrine (`webtyp/app/docs/CONSTRUCTION_HARNESS.md`):
  it is prose the compiler cannot verify, a mandatory step authors must
  remember, and a **second copy** of a fact `Storage()` already states in
  typed code — the copies can silently contradict each other
  (`//ormc:storage int` above a text kind compiles fine and generates wrong
  code).
- Was partially implemented (directives briefly existed in
  `webtyp/input`'s working tree, never published; a cloud agent
  implementing the ormc side was stopped and discarded).

### 2. Embedded storage-marker types — REJECTED

`model` would export zero-size types (`model.TextStorage`, …) whose promoted
method is `Storage()`; kinds embed one; ormc finds the embed via AST.

- Solves the divergence problem (marker IS the runtime method), but adds
  API to `model` that exists only to serve the generator's limitation —
  boilerplate in the contract package.
- Requires a heuristic AST walk in ormc (constructor → concrete return type
  → embed chain, across packages): real fragility, where an unhandled
  authoring shape rejects valid code. The "compile-time guarantee" is
  partial — resolvability is only checked at generation time.
- Blast radius: a four-repo wave (model, form, ormc, modfind) for one datum.

### 3. Dependency probe — CHOSEN

Generate a throwaway `main` importing the kinds' packages, execute each
captured constructor, read the real `Storage()`, cache the results.

- **Zero authoring surface**: implementing `model.Kind` is all a kind ever
  needs. Nothing to remember, no possible divergence — the probe executes
  the single source of truth.
- **The compiler is the resolver**: no heuristics; any authoring shape that
  compiles works.
- **Consistent with the §9 prohibition**, which protects the *user's*
  package: the probe imports only **dependency** packages, which always
  compile (they are ordinary requires of the scanned module). §9 was
  amended with this nuance.
- Only ormc changes — no new API anywhere else.

## Accepted costs and the rules they impose

| Cost | Rule that contains it |
|---|---|
| Probe needs the Go toolchain at generation time | ormc already shells `go list` via modfind; same environment |
| `go run` latency (~1s) on cache miss | cache keyed by (module root, `go.mod` hash, constructor set); watcher saves hit the cache |
| A kind in the same package as its Definitions re-creates chicken-and-egg | hard error: custom kinds live in their own package |
| Constructor args referencing user-package identifiers can't be probed | hard error: self-contained literal arguments only |
| Probe executes dependency code at generation time | same trust model as `go generate`; kind constructors are stateless zero-arg prototypes |
| Custom composition kinds would need ref plumbing the probe can't see | unsupported by design: composition is exclusive to `model.Struct(ref)`/`StructSlice(ref)` |
