---
PLAN: "refactor!: supply_manager on the crud-by-declaration framework — Allow and Rules in definition.go, persistence by crud, only domain rules in rule files"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 943936511984720505
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> **Publishing:** you publish only by OPENING THE PULL REQUEST (a `git push` from your sandbox does not reach GitHub). Open it after Stage 3 with whatever compiles and keep committing to it.
> **Never ask a yes/no question about progress**; ask only when a signature in this plan is wrong, and name the symbol.
> Never edit `definition_gen.go`: a build error there means `definition.go` or a rule file is wrong; fix that and run `definitionc` again.


# Plan: `supply_manager` on the crud-by-declaration framework

Today `ops.go` mounts 16–20 operations by hand. The four stock actions (`receive_stock`, `register_entry`, `register_output`, `transfer_stock`) share one permission (movement/Create) and write several tables (batches, levels, movements) with first-expiry-first-out allocation from `webtyp.com/stock`. After this plan a stock action is **one record created** in a new table `SupplyOperation` (D18), and its `Create` rule runs today's logic inside the transaction.

Reference implementation of the same migration (read it first, copy its shape): `patient_directory` — `definition.go` (`Definition`, `Allow`, `Rules`), `rule_patient_create.go`/`rule_patient_update.go`/`rule_patient_search.go` (the rules), `store.go`, `readers.go` (signatures kept, bodies on `m.store`).

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go install webtyp.com/definitionc/cmd/definitionc@v0.1.43
go get webtyp.com/model@v0.2.20 webtyp.com/crud@v0.0.14 webtyp.com/orm@v0.12.33 webtyp.com/storage@v0.2.2 webtyp.com/ddl@v0.0.49 webtyp.com/router@v0.4.22 webtyp.com/definitionc@v0.1.43 webtyp.com/view@v0.6.74 webtyp.com/appkit@v0.2.36 webtyp.com/rpc@v0.2.21
```
Remove every `replace` from `go.mod`; `go mod tidy`.
`definitionc` is installed in `$(go env GOPATH)/bin`, which is not on the PATH of a fresh shell: write `export PATH="$PATH:$(go env GOPATH)/bin"` in the SAME command that runs it. **`definitionc` rewrites `go.mod` to older versions on every run: after EVERY run repeat the `go get` line above and `go mod tidy`.** If a module cannot be fetched, stop and say so in the PR; never write a hash by hand.
Before any change check, with `go doc`, that these exist: `model.ModuleDefinition`, `model.Rules` (`model.AddMyBusinessRule`, `model.HaveNoBusinessRule`), `crud.CurrentUser`, `crud.RuleAnswer`, `crud.Topic`, `crud.LoopbackActingAs`, and that `definitionc` writes a `definition_gen.go` containing `func (m *Module) MountOperations(`. If one is missing, stop and say so in the PR.

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

A rule ends with `return crud.ApplyAndPublish(), nil` (saves and announces) or `return crud.ApplyWithoutPublishing(), nil` (saves without announcing); a rejection is `return crud.RuleAnswer{}, err`. It queries with `tx`, never with `m.db`. To write another row it calls `tx.Create`/`tx.UpdateFields`. The old `Committed` hook does not exist: `crud` publishes.

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
| `list_*/get_* of products/items, locations, batches, stock, movements` | `<table>.read` (with `id` for one) |
| `create_*/update_*/upsert_* of products/items, locations` | `<table>.create` / `<table>.update` |
| `deactivate_*` | `<table>.update` with `["is_active"]` |
| `delete_*` | `<table>.delete` |
| `receive_stock` | `supply_operation.create` with `kind: "receive"` |
| `register_entry` | `supply_operation.create` with `kind: "entry"` |
| `register_output` | `supply_operation.create` with `kind: "output"` |
| `transfer_stock` | `supply_operation.create` with `kind: "transfer"` |
| `list_expiring` | read model `ExpiringStock` |
| `list_low_stock` | read model `LowStock` |

## Stages (do them in order; commit after each)

| Stage | File |
|---|---|
| 1 — `definition.go` | [PLAN_STAGE_1_definition_go.md](PLAN_STAGE_1_definition_go.md) |
| 2 — `module.go` | [PLAN_STAGE_2_module_go.md](PLAN_STAGE_2_module_go.md) |
| 3 — generate, then write the rules | [PLAN_STAGE_3_generate_then_write_the_rule.md](PLAN_STAGE_3_generate_then_write_the_rule.md) |
| 4 — readers used by other modules | [PLAN_STAGE_4_readers_used_by_other_module.md](PLAN_STAGE_4_readers_used_by_other_module.md) |
| 5 — browser side (D15, D16) | [PLAN_STAGE_5_browser_side.md](PLAN_STAGE_5_browser_side.md) |
| 6 — tests (`tests/`, through `crud.LoopbackActingAs(nil, "u1", mod)`; rows in `docs/TESTS.md`) | [PLAN_STAGE_6_tests.md](PLAN_STAGE_6_tests.md) |
| 7 — translations and documentation (Spanish, veltylabs) | [PLAN_STAGE_7_translations_and_documentati.md](PLAN_STAGE_7_translations_and_documentati.md) |

## Delivery check

`gotest` passes; `langc sync .` clean; every grep of this plan empty. A tree that does not compile or whose tests did not run is a
failed delivery.
