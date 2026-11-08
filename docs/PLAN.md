---
PLAN: "refactor!: supply_manager on the closed crud contract — server tenant, transactional batch, single declarative validation, view.go browser-only"
EXECUTOR: jules
REVIEWER: none
---

> Read AGENTS.md at the repo root before any change.
> **Finish by yourself:** when the delivery check passes, commit, push and open the pull request. Never ask whether to
> proceed or finalize — nobody answers, and a session that waits ends without your work. Commit after every stage.
> Never run `git reset`, `git restore`, `git checkout -- <file>` or `git stash` on your own work.
> Never edit `model_orm.go` or `model_crud.go`: a build error there means the hand-written code is wrong.

# Plan: `supply_manager` on `crud.Registry`

Context: master `webtyp/docs/ARCHITECTURE_MASTER_PLAN.md` D2–D16 (phases A4.2 + L5). Follows the template in `patient_directory`.
After this plan the module mounts operations through generated helpers on `crud.Registry`,
tenant comes from `Deps`, validation of the schema lives in one place (`crud`), and `seed.Demo` provides the demo.

## Tools and local dependencies

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go install webtyp.com/ormc/cmd/ormc@v0.1.36
```
```bash
go get webtyp.com/model@v0.2.14 webtyp.com/router@v0.4.19 webtyp.com/crud@v0.0.3 webtyp.com/rpc@v0.2.17 webtyp.com/mcp@v0.3.23 \
  webtyp.com/dbsync@v0.3.47 webtyp.com/view@latest webtyp.com/appkit@latest
```
`view@latest` and `appkit@latest` contain `view.Demo` and `appkit` without `layout` (plans L1, L2).
Remove every `replace` from `go.mod`; `go mod tidy`; run `ormc` (generates `model_crud.go`).
After the delivery check, `go.mod` does not list `webtyp.com/layout`.

## APIs used

| Symbol | Signature |
|---|---|
| generated `Mount<Entity>Read/Create/Update/Delete` | typed signatures on `crud.Registry` |
| `crud.ReadArgs` | `struct{ ID, Search, After string; Limit int64; Filter model.Model }` |
| `crud.Loopback` / `crud.LoopbackActingAs` | `func Loopback(mods ...crud.Module) router.Caller` / `func LoopbackActingAs(userID string, mods ...crud.Module) router.Caller` |
| `crud.Op` | `func Op(m model.ModuleNaming, action model.Action) string` |
| `model.ValidateFields` | `func ValidateFields(action byte, f Fielder) error` |
| `orm.DB.Tx` | `func (d *DB) Tx(fn func(tx *DB) error) error` |

## Explicit deletions (grep must be empty)

- `ops.go`: raw `reg.Operation` calls, `var _ router.OperationModule`, imports `webtyp.com/router`, `webtyp.com/rpc`, `webtyp.com/view`.
  `grep -rn "reg.Operation\|router.Context\|rpc\.\|args.TenantId" --include=*.go . | grep -v _test` → empty.
- `model.go`: manual argument models for ops if any; errors covered by schema/crud.
- `module.go`: operations logic moved to `crud.go` and `queries.go`.

## Stage 1 — `model.go` and `errors.go`

- `tenant_id` and `updated_at` get `ServerOnly: true` on models.
- Verify `errors.go` has doc comments in Spanish with usage examples. Run `ormc`.

## Stage 2 — `crud.go` (writes) and `queries.go` (reads); `module.go` keeps core `Deps`, `Module`, `New`

- `crud.go`: implement Create, Update, Delete for domain entities wrapped in `db.Tx`, filling `tenant_id` from `m.tenantID`.
- `queries.go`: implement Read with `crud.ReadArgs` (supports ID lookup, Search, After, Limit). Always scope by `tenant_id`.

## Stage 3 — `ops.go`

- `MountOperations(reg crud.Registry)` mounts operations via generated helpers.
- `Module` implements `crud.Module` (`model.ModuleNaming` + `MountOperations(reg crud.Registry)`).

## Stage 4 — `view.go`, `seed/demo.go`, delete `web/` if present, `go.mod`

- `view.go`: add `//go:build browser`. Update `view.Ops` to use `model.Action`:
  `view.Ops{Module: ModelName, Allow: model.Create | model.Read | model.Update | model.Delete, Page: view.Page{First: view.All}}`
- `seed/demo.go`: provide `Demo() (view.Demo, error)` returning `view.Demo{Label: ui.DefaultLabel, Timezone: "America/Santiago", ActingAs: "u1", Modules: []crud.Module{mod}, Screens: func(d view.BrowserDeps) []view.Screen { return ui.Screens(d.Caller) }}`.
- `go mod tidy`: ensure `webtyp.com/layout` is not present in `go.mod`.

## Stage 5 — tests (`tests/`)

- Update tests to use `crud.Loopback` / `crud.LoopbackActingAs("u1", mod)` and `crud.CreateArgs`, `crud.ReadArgs`, `crud.UpdateArgs`, `crud.DeleteArgs`.
- Operation names derived from `crud.Op(model, action)` qualified with module ModelName.
- Add `TestDemo_Valid` verifying `seed.Demo()`.

## Stage 6 — translations and documentation

- `langc sync .`: zero untranslated, zero warnings.
- Update `README.md` and `docs/ARCHITECTURE.md`.

## Delivery check

`gotest` passes; `langc sync .` clean; every grep above empty.
