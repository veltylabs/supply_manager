---
PLAN: "refactor!: supply_manager on the crud-by-declaration framework — Allow and Rules in definition.go, persistence by crud, only domain rules in rule files"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> Never edit `definition_gen.go`: a build error there means `definition.go` or a rule file is wrong; fix that and run `definitionc` again.
> This plan is dispatched in one wave with the other 11 modules without external calls (D53), after `patient_directory` (the reference) was executed and approved by the owner.


# Plan: `supply_manager` on the crud-by-declaration framework

Today `ops.go` mounts 16–20 operations by hand. The four stock actions (`receive_stock`, `register_entry`, `register_output`, `transfer_stock`) share one permission (movement/Create) and write several tables (batches, levels, movements) with first-expiry-first-out allocation from `webtyp.com/stock`. After this plan a stock action is **one record created** in a new table `SupplyOperation` (D18), and its `Create` rule runs today's logic inside the transaction.

Reference implementation of the same migration (read it first, copy its shape): `patient_directory` — `definition.go` (`Definition`, `Allow`, `Rules`), `rule_patient_create.go`/`rule_patient_update.go`/`rule_patient_search.go` (the rules), `store.go`, `readers.go` (signatures kept, bodies on `m.store`).

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go install webtyp.com/definitionc/cmd/definitionc@v0.1.41
go get webtyp.com/model@v0.2.18 webtyp.com/crud@v0.0.11 webtyp.com/orm@v0.12.30 webtyp.com/storage@v0.2.1 webtyp.com/ddl@v0.0.49 webtyp.com/router@v0.4.21 webtyp.com/definitionc@v0.1.41 webtyp.com/view@v0.6.72 webtyp.com/appkit@v0.2.33 webtyp.com/rpc@v0.2.21
```
Remove every `replace` from `go.mod`; `go mod tidy`. Before any change check, with `go doc`, that these exist: `model.ModuleDefinition`, `model.Rules` (`model.AddMyBusinessRule`, `model.HaveNoBusinessRule`), `crud.CurrentUser`, `crud.RuleAnswer`, `crud.Topic`, `crud.Loopback`, and that `definitionc` writes a `definition_gen.go` containing `func (m *Module) MountOperations(`. If one is missing, stop and say so in the PR.
Prerequisite of the whole wave: `patient_directory` was executed, reviewed and approved by the owner; it is the reference and the 17 other modules are dispatched together after it.

## Lessons of the reference module (`patient_directory`, executed locally)

- Needs `crud` ≥ v0.0.11 (a filter has no required field; v0.0.10 rejected every read of a table with a rule). `definitionc` ≥ v0.1.41 generates `Search(a, filter *X, text, m)`; a text of `""` is normal (the rule runs on every read).
- Running `definitionc` rewrites `go.mod` to older versions: run the `go get` list of «Tools» again after it, then `go mod tidy`; raise stale indirect dependencies (`rbac`, `mcp`, `appkit`) to the published ones.
- In tests, events reach the publisher given to `crud.LoopbackActingAs(pub, "u1", mod)`, not `Deps.Publisher`; a caller double takes `router.Operation`; a `ddl.Execer` double returns `(storage.Result, error)`; a rule that answers `0` fails with an internal error (assert the refusal and that nothing was stored).
- `langc sync .` can warn that other languages are untranslated: that is the repo's `lang.json`, not this plan.

## The rule of this plan (decisions D28–D50 of the architecture master)

The framework persists; the module declares and writes only domain rules.

| The module no longer writes | Who does it now |
|---|---|
| id, `tenant_id`, `created_at`/`updated_at`, the transaction loop, create/update/delete bodies, «not found», batch atomicity, idempotent retry by id, paging by `After`, `tenant_id` in every query, schema checks repeated from the model | `crud` + `orm.TenantStore` / `orm.GlobalStore` |
| an operation per verb (`list_*`, `save_*`, `upsert_*`, `delete_*`), its handler, `ops.go`, the permission resources and the event topics | `Allow` + `Rules` in `definition.go`; `definitionc` writes `MountOperations`; `crud` names the operation (`router.Operation{Module, Entity, Action}`; on the wire `GET\|POST\|PATCH\|DELETE /api/<module>/<entity>`, in MCP `<module>.<entity>.read\|create\|update\|delete`) and publishes `<module>.<entity>.created\|updated\|deleted` with a `*crud.Change` after the commit |
| an action with its own permission (approve, settle, receive stock…) | a **record created** in its own table, or a **read model** with its own name (D18) |

A module writes: `definition.go` (`var Definition = model.ModuleDefinition{…}` and, per table, `Allow` and `Rules`), one `rule_<entity>_<verb>.go` for every action declared `model.AddMyBusinessRule` (created once by `definitionc`; the skeleton does not compile until completed) and `store.go` (`func (m *Module) Store<Entity>() crud.Store`, one line per table). Every allowed action that has no rule below is declared `model.HaveNoBusinessRule`: `crud` alone checks it. Read the reference first: `patient_directory` (`definition.go`, `rule_patient_*.go`, `store.go`).

| Rule | Form (the receiver is the entity) |
|---|---|
| Create | `func (p *X) Create(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error)` |
| Update | `func (next *X) Update(a crud.CurrentUser, tx crud.Tx, old *X, fields []string, m *Module) (crud.RuleAnswer, error)` (`next` carries only the sent `fields`) |
| Delete | `func (p *X) Delete(a crud.CurrentUser, tx crud.Tx, m *Module) (crud.RuleAnswer, error)` (`p` is the stored row) |
| Search (Read of a table) | `func (p *X) Search(a crud.CurrentUser, filter *X, text string, m *Module) ([]storage.Condition, error)` — AND conditions only; it runs on every read, `text` may be `""`, and `filter` is a record of the entity with only the fields the caller sent (D52: the entity is its own filter) |
| Read (model without a table) | `func (p *X) Read(a crud.CurrentUser, filter *X, args crud.ReadArgs, m *Module) (model.ModelSlice, error)` (`filter` carries the fields the caller sent, D52) |

A rule ends with `return crud.ApplyAndPublish, nil` (saves and announces) or `crud.ApplyWithoutPublishing` (saves without announcing); a rejection is `return 0, err`. It queries with `tx`, never with `m.db`. To write another row it calls `tx.Create`/`tx.UpdateFields`. The old `Committed` hook does not exist: `crud` publishes.

## Tables, stores and rules

An action with a rule below is declared `model.AddMyBusinessRule` in the table's `Rules`; every other allowed action is `model.HaveNoBusinessRule`. A `Search` below is the rule of `Read`.

| Table (`<X>Model`) | Store | `Allow` | Rules (`rule_<entity>_<verb>.go`) — where their rules come from today |
|---|---|---|---|
| `SupplyItem` | `orm.TenantStore` | `model.Create \| model.Read \| model.Update \| model.Delete` | `Create`/`Update`: the checks of `CreateItem`/`UpdateItem`/`UpsertItem`; deactivate = Update with `["is_active"]`; `Delete`: the checks of `DeleteItems` (no stock left) |
| `SupplyLocation` | `orm.TenantStore` | `model.Create \| model.Read \| model.Update \| model.Delete` | `Create`/`Update`: the checks of `CreateLocation`/`UpdateLocation`; `Delete`: the checks of `DeleteLocations` |
| `SupplyBatch` | `orm.TenantStore` | `model.Read` | none (written by operations) |
| `SupplyStockLevel` | `orm.TenantStore` | `model.Read` | none (written by operations) |
| `SupplyMovement` | `orm.TenantStore` | `model.Read` | none (written by operations) |
| `SupplyOperation (**new table**)` | `orm.TenantStore` | `model.Create \| model.Read` | `Create`: by `kind` call today's logic of `ReceiveStock`, `RegisterEntry`, `RegisterOutput`, `TransferStock` rewritten to read/write through the `crud.Tx` it receives (ordered reads with `crud.Query.Order`, e.g. `storage.Asc("expires_at")`); `actor_id` = `a.UserID`; the resulting movement ids are stored in `movement_ids` |

## Read models (D18 — no table, `Allow: model.Read` only, `Rules.Read: model.AddMyBusinessRule`)

| Model | Filter | Answered by |
|---|---|---|
| `ExpiringStock` (existing `ExpiringStockModel`; `Allow: model.Read`, no `DB:` marks) | `ExpiringFilter{{days int64}}` (from `ListExpiringArgsModel`) | the `Read` rule of `ExpiringStock` (`rule_<model name>_read.go`) — the fields of the filter `ExpiringFilter` become fields of this model and the rule reads them from its `filter` parameter (D52) — body of `m.readExpiring` → `ListExpiring` |
| `LowStock` (existing `LowStockModel`; same) | none | the `Read` rule of `LowStock` (`rule_<model name>_read.go`), body of `m.readLowStock` → `ListLowStock` |

A read model is a `model.Definition` **without** `DB:` marks, with `Allow: model.Read` and `Rules: model.Rules{Read: model.AddMyBusinessRule}`, listed in `Definition.Entities`; `definitionc` mounts it and creates its `rule_<model name>_read.go`. Its permission resource is its own `ModelName`: list it in the README permissions table (roles must be granted it).

## Operation mapping (every operation of today has exactly one row)

«After» is `router.Operation{Module, Entity, Action}` written `<entity>.<verb>`; the wire form is `crud`'s (REST `/api/<module>/<entity>` by method, MCP `<module>.<entity>.<verb>`).

| Today | After |
|---|---|
| `list_*/get_* of products/items, locations, batches, stock, movements` | `<table>_r` (with `id` for one) |
| `create_*/update_*/upsert_* of products/items, locations` | `<table>_c` / `<table>_u` |
| `deactivate_*` | `<table>_u` with `["is_active"]` |
| `delete_*` | `<table>_d` |
| `receive_stock` | `supply_operation.create` with `kind: "receive"` |
| `register_entry` | `supply_operation.create` with `kind: "entry"` |
| `register_output` | `supply_operation.create` with `kind: "output"` |
| `transfer_stock` | `supply_operation.create` with `kind: "transfer"` |
| `list_expiring` | read model `ExpiringStock` |
| `list_low_stock` | read model `LowStock` |

## Stage 1 — `definition.go`

- Set `Allow` on each table exactly as in «Tables, stores and rules»; a table not listed there gets no `Allow` (internal).
- Mark `ServerOnly: true`: `tenant_id`, `created_at`, `updated_at`, and every field the server fills (`actor_id`, `movement_ids`, `cost` and every computed amount).
- Delete every `*Args` / `*Result` definition that only carried a removed operation (keep `ReceiveStockArgsModel`, `RegisterEntryArgsModel`, `RegisterOutputArgsModel`, `TransferStockArgsModel` with `Wire: true` — the readers `ReceiveStock`… take them — and `ListExpiringArgsModel` renamed to `ExpiringFilterModel`, `Wire: true`)
  (`grep -n "ArgsModel\|ResultModel" model.go` → only the ones this plan keeps).
- New table `SupplyOperationModel` (name `supply_operation`) with: `id` (PK), `tenant_id`, `kind` (`input.Select()` with the four constants `OperationReceive = "receive"`, `OperationEntry = "entry"`, `OperationOutput = "output"`, `OperationTransfer = "transfer"`), and the union of the fields of `ReceiveStockArgsModel`, `RegisterEntryArgsModel`, `RegisterOutputArgsModel`, `TransferStockArgsModel` (same names and kinds, `OmitEmpty` where only some kinds use them), plus `actor_id`, `movement_ids` (`model.Text()`, comma-joined), `created_at`. Add it to `migrate/` (`Sync`). A field a kind needs and lacks → field error from the rule.
- **Keep every `*Args` model that an exported reader's signature still uses** (`grep -n "^func (m \*Module) [A-Z][A-Za-z]*([^)]*Args" *.go` lists them): mark each `Wire: true`; delete only the `*Args` no remaining signature uses.
- D52 (the entity is its own filter): a filter model (`*Filter`, `*FilterModel`, an `*ArgsModel` that only carried filter values) is not kept: its fields become fields of the read model or table (`OmitEmpty`), and the rule reads them from its receiver; delete the filter type and its `Wire`/`NewFilter` plumbing.
- Add `var Definition = model.ModuleDefinition{Name: "supply_manager", Entities: []model.Definition{…every table and read model of this plan…}}` and, on every table and read model, `Rules` as «Tables, stores and rules» says (`AddMyBusinessRule` where a rule is named, `HaveNoBusinessRule` for the other allowed actions; a read model: `Read: model.AddMyBusinessRule`). Delete the hand topics (`Topic*`) and `Err*`/resources no code uses any more.

## Stage 2 — `module.go`

`Deps` keeps its fields; add `IDs model.IDGenerator` (required, `depErr("IDs")` when nil) if it is missing.
`Module` gains `store *orm.Store`, built once in `New`: `m.store = orm.TenantStore(db, deps.TenantID, deps.IDs)`.
Delete the hand-written persistence (create/update/upsert/patch/delete/list/get bodies that only move rows) once its rules moved to the rule files: the row-moving bodies of every create/update/upsert/patch/deactivate/delete/list/get of products/items and locations; `ReceiveStock`, `RegisterEntry`, `RegisterOutput`, `TransferStock` keep their signatures only as readers (below) and call the same rule logic inside `m.store.Atomic`.

## Stage 3 — generate, then write the rules

1. Delete `ops.go`, `crud.go`, `queries.go`, `model_crud.go`, `model_orm.go` and `definition_crud.go` (those that exist). Run `definitionc`: it writes one `definition_gen.go` (the types, the schemas and, on `Module`, `ModelName()`, `Definition()`, `MountOperations()`) and the skeleton `rule_<entity>_<verb>.go` of every action declared `AddMyBusinessRule`.
2. Add `store.go`: for each table of «Tables, stores and rules» that has a Store, `// Store<Entity> is the store of <entity>.` `func (m *Module) Store<Entity>() crud.Store { return m.store }` (the module's global store for a table the plan lists with `orm.GlobalStore`).
3. Fill each skeleton with the body of the rule named in «Tables, stores and rules» (and «Read models»), in the forms of «The rule of this plan». An old rule `func(c crud.Caller, tx, rec) error` becomes the rule method with the same body, `c.UserID` becomes `a.UserID`, a nil error becomes `return crud.ApplyAndPublish, nil`, an error `e` becomes `return 0, e`.
4. Specific to this module: Every rule named above is a rule file. `runOperation(c, tx, op)` switches on `op.Kind` (unknown kind → field error on `kind`) and runs the moved logic with `tx`; `webtyp.com/stock` stays the only place with allocation arithmetic.
Delete every `Op*` constant, `res*`/`Resource*` variable, every `op*` handler, every `view.MountSave`/`view.MountDelete`/`reg.Operation` call and the `webtyp.com/router`, `webtyp.com/rpc` imports that only they used (`grep -rn "reg.Operation\|view.MountSave\|view.MountDelete\|router.Context\|rpc.WriteError" --include=*.go . | grep -v _test` → empty).

## Stage 4 — readers used by other modules

Signatures of every exported reader stay (other modules and apps call them). Bodies read through `m.store` (`ReadOne`, `ReadAll` with `crud.Query`), never `m.db` directly; a `tenantID` parameter different from the module's tenant answers «not found» (`found=false` / empty, `err=nil`). Keep `ItemExists`, `ItemCard`, `GetItem`, `GetLocation`, `GetBatch`, `ListItems`, `ListLocations`, `ListBatches`, `ListStock`, `ListMovements`, `ListExpiring`, `ListLowStock`, `ReviewExpiring`, `UseInRoom`, `ReceiveStock`, `RegisterEntry`, `RegisterOutput`, `TransferStock` (each builds the `SupplyOperation` record and runs `m.runOperation` inside `m.store.Atomic`).

## Stage 5 — browser side (D15, D16)

- `view.go` (`//go:build browser`): every `view.NewCallerLister(caller, view.Ops{…}, newList)` takes
  `func() model.TableSlice { return &<Entity>List{} }` and `view.Ops` has no `Allow` (the screen reads `<Entity>List.Allow()`).
- `module/browser.go`, `module/server.go` (if present): `appkit.ModuleBrowser`/`appkit.BrowserDeps` → `view.ModuleBrowser`/`view.BrowserDeps`.
- Demo: `seed/demo.go` (`//go:build wasm`) returning `view.Demo{Label, Timezone: "America/Santiago", ActingAs: "u1",
  Modules: []crud.Module{mod}, Migrations: <the module's ddl.Step list, or nil if it has no migrations>,
  Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(<what today's demo main passes>) }}`, built with
  `orm.New(mem.New())`, `New(db, Deps{…})` with the same fakes today's demo uses, and `seed.Load` (when the module has one).
  Delete the old demo: any `web/` demo (`test ! -e web`).
- `grep -rn "webtyp.com/layout\|appkit.BrowserDeps\|appkit.ModuleBrowser\|router/loopback" .` → empty, `go.mod` included.

## Stage 6 — tests (`tests/`, through `crud.LoopbackActingAs(nil, "u1", mod)`; rows in `docs/TESTS.md`)

Delete tests of removed code. Operations are `router.Operation{Module: ModelName, Entity: "<entity>", Action: model.<Action>}`, never a string; fixtures follow `crud` v0.0.10 (`reg.AddStoredEntity`, `crud.CurrentUser`). Add `TestMount_MatchesDefinition` (`crud.Mount(reg, nil, nil, mod)` returns nil) and one test that an allowed write publishes a `*crud.Change` on `crud.Topic(ModelName, "<entity>", model.<Action>)`. Required:

| Test | Setup → call → expected |
|---|---|
| `TestTenant_ReadOtherTenantIsNotFound` | product/item of tenant a read by tenant b → `model.ErrorNotFound` |
| `TestOperation_ReceiveCreatesBatchAndLevel` | create operation `receive` → one batch, stock level = quantity, one movement; operation `movement_ids` lists it |
| `TestOperation_OutputFirstExpiryFirst` | two batches expiring day 5 and day 2; output 3 → taken from the day-2 batch first |
| `TestOperation_OutputNeverNegative` | output more than stock → error, no movement, levels unchanged (atomic) |
| `TestOperation_TransferMovesBetweenLocations` | transfer 2 from A to B → level A −2, level B +2, two movements |
| `TestOperation_UnknownKindRejected` | kind "x" → field error on `kind` |
| `TestMovement_NoWriteOperations` | `<movement>_c` → unknown operation |
| `TestReadModel_LowStock` | item under its minimum → appears in `LowStock` |
| `TestDemo_Valid` (`//go:build wasm`) | `seed.Demo()` → no error, `Validate()` nil, at least one screen |

## Stage 7 — translations and documentation (Spanish, veltylabs)

`langc sync .` (zero pending, zero reuse warnings). `README.md` (≤150 lines, 4 mandatory sections; «Sin/Con»: hand-written
operations vs `Allow` + rules) with the operations table of the mapping above. `docs/ARCHITECTURE.md`: rules vs persistence.
Translate to Spanish every README, `docs/*`, diagram and code comment still in English; exported symbols without a usage example get one.

## Delivery check

`gotest` passes; `langc sync .` clean; every grep of this plan empty. A tree that does not compile or whose tests did not run is a
failed delivery.
