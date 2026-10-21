# AGENTS.md — veltylabs/modules (canonical template)

<!-- webtyp:base — copied from devskills skill agents-md; do not edit here -->
## Ecosystem rules — read before any change

`docs/PLAN.md` is the spec: no design judgment; anything unclear is a question in the PR, not a guess.

- Public API: typed (no `any`/`interface{}`), explicit, minimal; never export a symbol only a test uses.
- One intent, one path: the old path is deleted in the same change. No silent fallback, `TODO`, commented code or stub.
- Fail at compile time, else a loud diagnostic — never silently. Zero values deny.
- A defect in a dependency is reported, never wrapped or patched here.
- No string literals in logic: named constants. The same logic never lives in two repos: import the package the plan names; if you find an existing one the plan does not name that already does what you are about to write, say so in the PR instead of writing it.
- `cmd/*/main.go`: parse flags, inject, print/exit only. stdout = data, stderr = logs, exit codes are the contract.
- WASM code: `webtyp/fmt`, `webtyp/time`, `webtyp/json` instead of stdlib; no `map`; `dom.Element` embedded by value.
- `css.go`, `svg.go`, `js.go`, `html.go` carry `//go:build !wasm`; no `ssr.go`, no `front.go`.
- Build tags: `wasm`/`!wasm` only when the API does not exist on that platform; `browser`/`!browser` when the file belongs to another binary (`web/browser.go` → `browser.wasm`, `web/edge.go` `wasm && !browser` → `edge.wasm`, `web/server.go` `!wasm` → `server`).
- Code and keys in English; UI text is `lang.Text`, translations live in `lang.json`.
- Tests cover behaviour and domain rules, never the look (CSS, colours, markup order, class strings). In `tests/` (external package), real stack with fakes only at the edge; run `gotest`.
- Docs (`README.md`, `docs/*`) updated with the API; permanent docs never link `docs/PLAN.md`.
- Before the PR: `git diff --stat main` lists every file of the plan's stages, and nothing the plan did not order is deleted.
<!-- /webtyp:base -->

## This repo

Working notes for AI agents operating on any `veltylabs/<module>` repo. Every module gets its own
copy of this file at its root, adjusted only in the "Domain-specific notes" section at the bottom —
the rules above that line are **verbatim across every module**, do not fork them. If a rule here
turns out to be wrong for a module, the fix belongs here (and replicates outward), not as a local
exception in one module's copy.

Master plan (cross-repo status, whitelist rationale, dispatch order):
[`app-releases/docs/REUSABLE_MODULES_MASTER_PLAN.md`](https://github.com/webtyp/app/blob/main/docs/REUSABLE_MODULES_MASTER_PLAN.md).
Reference implementation (the pattern every module replicates): `github.com/veltylabs/item_catalog`.

## Mission of a `veltylabs/modules/*` repo

A **domain module**: business logic for one bounded concern (catalog, scheduling, payouts, …),
published as an independent Go library, importable by any app in the Velty ecosystem. It is a piece
of lego — it must assemble under a transport, a database, an ID generator, an encoding and a
renderer it has never seen, chosen by whichever app composes it.

## The whitelist — what a module may import

A module's **non-test** Go files may import, from `webtyp.com/*`:

| Package | Role | Why it's a port, not a concrete dependency |
|---|---|---|
| `model` | `Model`/`Fielder`/`Encodable`/`Decodable`/`IDGenerator`/`Definition` | Schema + codec *interfaces*; concrete encoders (`json`, `jsvalue`) live outside |
| `router` | `OperationModule`/`OperationRegistry`/`Context`/`Caller` | Transport-agnostic; a module implements `OperationModule`, never a concrete server |
| `view` | `Presenter`, `view.New(...)` | UI contract; the renderer (`layout/crudview` or any other) is injected by the app |
| `events` | `Publisher`/`Subscriber`/`Event` | Pub/sub contract; the broker (in-proc, `sse`, a queue) is injected |
| `stock` | `Movement`/`Direction`/`Lot`/`Allocate`/`Apply` | Pure stock rules (first-expiry-first-out, never-negative arithmetic); no I/O, so it is not a concrete dependency. Used by `pharmacy_manager` and `supply_manager` |
| `money` | `Money`/`Fact`/`Publish` | Pure money values and the accounting-ready facts a module announces through `events`; no I/O. Used by modules that move money or stock at cost |
| `orm` | `*orm.DB`, query builder (`Create`/`Update`/`Delete`/`Query`) | Ergonomic layer over `storage.Conn` — the equivalent of `database/sql`, backend-agnostic by construction |
| `storage` | `Conn`/`Condition`/`Query` (mostly transitive, via `orm`'s re-exports in `reexport.go`) | The actual storage **port** — `orm` never redefines it |
| `ddl` | `CreateTable`/`Sync` (runtime schema) | Sibling of `orm` over `storage.Conn` — still backend-agnostic |
| `form/input` | `input.Text()`, `input.Number()`, … (`Kind` with a UI widget) | Only when a `model.Definition` field needs a widget for a form; still just a `Kind`, no renderer |
| `fmt` | string/number/error handling, `Err(...)` | The ecosystem's stdlib replacement — see the stdlib section below |
| `time` | date/time helpers | The ecosystem's `time` replacement (isomorphic, wasm-safe) |

**"Port" vs "concrete implementation" is the whole test.** `orm`/`storage`/`ddl` are agnostic — they
work unchanged against `sqlt`, `postgres`, `mem`, or a future `indexdb` backend, selected by whoever
calls `orm.New(conn)`. A module importing them does **not** know or care which backend is behind the
`storage.Conn` it receives. That is different from importing `webtyp/sqlite` (a driver) or
`webtyp/mcp` (a transport) directly — those name one implementation, foreclosing all the others.

## The blacklist — what a module may never import, in any file, tests included

- **Any concrete storage backend**: `webtyp/sqlite`, `webtyp/sqlt`, `webtyp/postgres`,
  `webtyp/indexdb`, or any `database/sql` driver. A module's own tests exercise
  `storage/mem` (`webtyp.com/storage/mem`) via `orm.New(mem.New())` — the in-memory
  reference backend built for exactly this. **Not even in `_test.go` files** — a concrete driver
  pulled in "just for tests" is still a dependency the module ships, and it is exactly the coupling
  this whitelist exists to prevent. Backend integration tests belong to the composition-root app
  repo, never to the module.
- **A concrete transport**: `webtyp/mcp`, `webtyp/server`/`httpd`, or anything importing
  `net/http`. A module speaks `router.OperationModule`; the app decides which transport harvests it.
- **A concrete ID generator**: `webtyp/unixid`. Accept `model.IDGenerator` via `Deps` instead —
  never construct one inside the module.
- **A concrete encoder**: `webtyp/json`, `webtyp/jsvalue`. A module's models implement
  `model.Encodable`/`Decodable` (generated by `ormc`); which concrete `FieldWriter`/`FieldReader`
  walks them (JSON text, JS values, a future binary codec) is the app's choice, made at the transport
  boundary, never inside the module.
- **A concrete renderer**: `webtyp/layout` (or any other UI kit). A module builds its
  `view.Presenter` with `view.New(caller, ...)` — `view`+`model`+`router` only. The app picks the
  renderer that draws that `Presenter`.
- **A self-declared port that duplicates an ecosystem contract**: no local `EventPublisher`,
  `UIAdapter`, `IDGenerator`, or `CatalogService`-as-transport-shim interface that intersects
  `events.Publisher`/`view.Presenter`/`model.IDGenerator`/`router.OperationModule`. If a boundary needs a
  contract this list doesn't name, that is a defect **upstream** (in `model`/`router`/`view`/`events`/
  `orm`), fixed there and consumed here — never patched locally. A module may still declare its own
  narrow cross-module reader interfaces (`CatalogReader`, `StaffReader`, …) for **domain** data it
  needs from a sibling module — that pattern stays (see Cross-module wiring below).
- **Model definitions**: Schema definition using `model.Definition`. Cross-module references stay soft (a plain string id/SKU) — the root domain package never imports another module (see "ui/, seed/ and web/" for the DAG rule).
- **No `internal/` package that forks or vendors an ecosystem repo.** If an upstream package is
  missing a function, the fix is a `docs/PLAN.md` against *that* repo, published upstream — never a
  local copy with a `replace` directive in `go.mod`. A `replace` pointing at a local path is always a
  defect to close, not a workaround to keep.

## The stdlib boundary

`webtyp.com/fmt` **is** the stdlib replacement for this ecosystem: nothing in it, and
nothing built on it, imports `fmt`, `strings`, `strconv`, or `errors` — `webtyp.com/fmt` already
provides string manipulation, number conversion, and error construction (`fmt.Err(...)`),
reflection-free and TinyGo-sized. A module targets `wasm`/TinyGo first, so it follows the same rule:

- **Banned, use the webtyp replacement instead:** `errors`, `strings`, `strconv`, stdlib `fmt` →
  `webtyp.com/fmt`. `encoding/json` → `model.Encodable`/`Decodable` (the module never
  chooses the concrete encoder; in tests, use `"webtyp.com/json"` for codec verification, standard library `"encoding/json"` is strictly banned everywhere). `database/sql` → `orm`/`storage`. `net/http` → `router`. `time` →
  `webtyp.com/time`.
- **Fine to use directly:** anything that isn't a contract this ecosystem already replaces —
  `testing`, `sort`, `context` (stdlib, when it's genuinely cancellation/deadlines, not the
  transport-level `webtyp/context`), etc. When in doubt: if `webtyp/fmt` (or another package in
  the whitelist above) already covers it, use that; if it doesn't, plain stdlib is fine.
- **No Go `map[K]V` anywhere**, test code included. TinyGo's map runtime is heavy and the cost lands
  on every wasm binary that imports the module, transitively. Use `fmt.KeyValue{Key, Value string}`
  for a string→string pair, or a small slice-of-structs scanned linearly for anything else — module
  collections (a tenant's items, a schema's fields) are always small enough that a linear scan costs
  nothing measurable.
- **No `reflect`.** Struct introspection is a build-time (`ormc`) concern; the module consumes
  generated `Schema()`/`Pointers()`/`EncodeFields`/`DecodeFields`, it never inspects itself at
  runtime.

## Identity, persistence, transport, view, events — the shape every module takes

- **Identity**: `Deps.IDs model.IDGenerator`, required. The module calls `m.ids.NewID()`; it never
  constructs a generator.
- **Persistence**: `New(db *orm.DB, deps Deps)` receives an already-connected
  `*orm.DB` and assumes its schema already exists — it never creates or
  alters tables, and never imports `webtyp.com/ddl`. Schema reconciliation
  lives in its own **subpackage**, `<module>/migrate` (`package migrate`),
  exporting `Migrate(conn ddl.Execer, ddlCompiler ddl.Compiler) error`.
  Deliberately not called by `New`, and deliberately not in the module's
  root package: schema work is deploy-time work, run once from a migration
  binary (`cmd/migrate` in the composition-root app) — and keeping it in a
  separate package means nothing on a WASM build's import path (`view.go`,
  `init.go`, the root package itself) ever pulls `webtyp.com/ddl` into that
  binary, regardless of build tags.
  ```go
  // migrate/migrate.go
  package migrate

  import (
      "webtyp.com/ddl"

      thismodule "github.com/veltylabs/<this-module>"
  )

  func Migrate(conn ddl.Execer, ddlCompiler ddl.Compiler) error {
      d := ddl.New(conn, ddlCompiler)
      if err := d.CreateTable(&thismodule.CatalogItem{}); err != nil {
          return err
      }
      return nil
  }
  ```
  A module's own tests build `*orm.DB` over `storage/mem`
  (`orm.New(mem.New())`), which creates tables lazily on first `Exec` — they
  never call `Migrate`, and `New` never needs to type-assert for
  `ddl.Compiler` at all anymore.
- **Transport**: the module implements `router.OperationModule` — `ModelName() string` +
  `MountOperations(reg router.OperationRegistry)`, registering each operation with `.Requires(resource, action)`
  and `.Accepts(&ArgsType{})`. It never implements `router.APIModule`/`Router`, and never sees
  `mcp.Tool`/`mcp.ToolProvider`.
- **View**: `NewView(caller router.Caller) view.Presenter`, built with `view.New(...)` — importing
  only `view`+`model`+`router`. The app supplies both the `router.Caller` and the renderer that draws
  the resulting `Presenter`.
- **Events**: `Deps.Publisher events.Publisher`, optional — `nil` disables publishing silently. The
  module publishes `events.Event{Topic: ..., Payload: &typedRecord}` after every successful mutation,
  never a bare `map` or `any` payload.
- **Cross-module wiring**: when module A needs data from module B, A declares the narrow interface it
  needs (`CatalogReader`, `StaffReader`, …) in its own package; B's `*Module` satisfies it structurally
  (no import of A). The composition root wires concrete instances together. Root domain packages never import each other; only ui/, seed/ and web/ may, upstream only.

## ui/, seed/ and web/ — the module's own view and demo

The whitelist and blacklist above apply to the **root domain package**. Three sub-packages are exempt, and only them:

- `ui/` (package `ui`, no build tag; `css.go`/`svg.go` tagged `!wasm`) may import `webtyp.com/layout/*`, `webtyp.com/components/*`, `dom`, `html`, `css`, `svg`, `widget`. It is the module's screen: `const ID`, `const Label` and `Browser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error)`.
- `seed/` (package `seed`, no build tag) holds demo data: `Load(...) (Data, error)`, which writes through the module's own methods so every row is validated, and returns the rows it created so downstream demos can reference them.
- `web/` (package `main`, `web/client.go` tagged `wasm`) is the runnable demo and may additionally import the concrete `webtyp.com/storage/mem`, `webtyp.com/router/loopback`, `webtyp.com/events/mock`, `webtyp.com/unixid` and `webtyp.com/auth/trusted_ip` (for `ValidateRUT`).

**Dependencies between modules form a DAG.** The root domain package still imports no sibling module. `ui/`, `seed/` and `web/` may import the domain, `ui/` and `seed/` packages of **upstream** modules only. Current graph: `device_manager`, `item_catalog`, `patient_directory`, `business_calendar`, `payment_manager` (leaves) ← `staff_manager` ← `appointment_booking` ← `clinical_encounter` (`appointment_booking` also depends on `item_catalog`, `patient_directory`, `business_calendar`, `payment_manager`; `clinical_encounter` also on `patient_directory` and `staff_manager`). A screen that mixes modules lives in the most-downstream module it touches.

## Testing

- Runner: `gotest`, never `go test` directly (once installed via
  `go install github.com/webtyp/devflow/cmd/gotest@latest`).
- A module's own tests build its `*orm.DB` over `storage/mem` (`orm.New(mem.New())`), drive
  `MountOperations` against `router/mock` (satisfies `router.OperationRegistry`), and exercise the `view.Presenter`
  against `view/conformance`'s `FakeCaller` or a hand-rolled fake `router.Caller` — never a concrete
  DB, transport, or renderer.
- Tests live in `tests/` (package `tests`, external — exercises only the exported API), per the
  ecosystem convention. Conformance tests must be in a separate test file with the same name (e.g., `conformance_test.go`), keeping test files lightweight and modular.
- Compile-time contract checks belong next to the implementation: `var _ router.OperationModule =
  (*Module)(nil)`.

## Publishing / dispatch

Changes to a `veltylabs/modules/*` repo go through the CodeJob workflow (see skill
**agents-workflow**): write a self-contained `docs/PLAN.md` with frontmatter (`PLAN`/`TAG`/
`EXECUTOR`/`REVIEWER`), the human dispatches it (`codejob`), an executor agent opens a PR, an
optional reviewer agent judges it, and closing the loop (`codejob 'msg'` or merging the PR) calls
`gopush` internally . A planning/authoring agent **writes** the plan; it
never runs `codejob` or `gopush` itself — dispatch and close are the human's call.

## Documentation a module must carry

| File | Purpose |
|---|---|
| `AGENTS.md` | This file, copied verbatim + a "Domain-specific notes" section below the line |
| `docs/ARCHITECTURE.md` | Domain scope, entities, the patterns above applied to this module, Ops table, composition-root example |
| `docs/PLAN.md` | Present only while a change is in flight — self-contained, deleted once merged (never left around "for reference") |
| `docs/diagrams/database.md` | Mermaid ERD |
| `README.md` | Quick start, Ops table, key files |

---

## Domain-specific notes (edit per module — nothing above this line)

`supply_manager` holds the clinic's **clinical supplies** (gloves, syringes, gauze…): what exists, the batches they
come in (a supply may have no lot), where they are kept and every movement of stock. The **pharmacy** is a different
module (`pharmacy_manager`): different managers, different permissions, different rules. The two share the rules in
the library `webtyp.com/stock` (first-expiry-first-out, movement directions, expiry predicates) and its conformance
suite; they do not share tables or code, and neither imports the other.

- Decision record: `CATALOG_SPLIT_MASTER_PLAN.md` in `webtyp/docs` (D1–D13).
- Supplies are **not sold and have no price** here.
- A batch's expiry is optional; a supply with no lot uses a batch whose number is the constant `NoBatchNumber`.
- A movement is **append-only**: a mistake is corrected by a new movement.
- Stock never goes negative. Supplies used in a consultation are recorded by `clinical_encounter` as `INTERNAL_USE`
  outputs that carry the encounter id (`source_document_type` / `source_document_id`).
