---
PLAN: "refactor: module review — errors via rpc.WriteError, resources from tables, screens via view.MountSave, foreign keys, translations"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 1820553771360114837
---

> Read AGENTS.md at the repo root before any change.

# PLAN — `supply_manager` passes the module review

## Why

Before a module enters an app it passes the 6-point review of skill **app-modules** («Before a
module enters an app»), and it adopts the pieces the framework shipped on 2026-10-29 (decisions
D22, D26, D27 of `veltylabs/mjosefa-cms/docs/MVP_DEPLOYMENT_ROADMAP.md`). Measured in this repo:

| Point | State today |
|---|---|
| Errors | domainError + writeError, 4 `==`; hand-declared `{records}` args in `ops.go` (delete them) |
| Permissions | 2 `Permission: "…"` literals |
| Save/delete of screens | the screen sends `{"records":[…]}`/`{"ids":[…]}`; these ops must read exactly that: `OpUpsertItem`, `OpUpsertLocation`, `OpDeleteItem`, `OpDeleteLocation` |
| Records `Item()` | present |
| Foreign keys | 10 `Ref` — keep |
| Translations | es filled, 7 empty (371 values) |
| Demo | present |

## APIs used (exact, from the published source)

| Symbol | Exact | Tag |
|---|---|---|
| `view.MountSave` | `func MountSave[T any, PT interface{ *T; model.Model }](reg router.OperationRegistry, op string, save func(PT) error) router.Route` | view v0.6.53 |
| `view.MountDelete` | `func MountDelete(reg router.OperationRegistry, op string, remove func(ids []string) error) router.Route` | view v0.6.53 |
| `rpc.WriteError` | `func WriteError(ctx router.Context, err error)` — status from the error kind; a 500 never sends `err.Error()` | rpc v0.2.10 |
| `model.ResourceOf` | `func ResourceOf(def *Definition) Resource` — the permission of a table (`""` if the definition has no PK) | model v0.2.12 |
| `model.FeatureResource` | `func FeatureResource(name string) Resource` — a permission that is not a table; declare it once as a package `var` | model v0.2.12 |
| `model.NotFound`, `model.Conflict`, `model.Invalid` | `type X string` with `Error()` and `Kind()`; `const ErrX model.NotFound = "…"` | model v0.2.12 |
| `model.FieldError` | `struct { Field string; Err error }` — a validation failure of one field | model v0.2.12 |
| `model.KindOf` | `func KindOf(err error) ErrorKind` (`ErrorInternal`, `ErrorInvalid`, `ErrorNotFound`, `ErrorConflict`) | model v0.2.12 |

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
go install webtyp.com/ormc/cmd/ormc@latest
go get webtyp.com/view@v0.6.55 webtyp.com/rpc@v0.2.12 webtyp.com/model@v0.2.12 webtyp.com/router@v0.4.16
```

## Stage 1 — errors

Delete the module's own `domainError`, `ValidationError`, `statusOf`/`mapErrorStatus` and
`writeError`. Re-type every `Err*` constant, **keeping its text and today's status**: 404 →
`model.NotFound`, 409 → `model.Conflict`, 400 → `model.Invalid`. Add
`ErrCannotRead model.Invalid = "supply_manager: cannot read the request"` for decode failures. A validation
wrapper around a model `Validate` error returns that error as is (it is already a
`*model.FieldError`); a hand-made field check becomes `&model.FieldError{Field: "<column>", Err: …}`.
Every handler answers errors with `rpc.WriteError(ctx, err)` — no `ctx.WriteStatus(4xx/5xx)` and no
`ctx.Write(err.Error())` left. Replace every `==`/`!=` between errors with `model.KindOf(err) ==
model.Error…` (it pulls reflection into the browser binary otherwise).

## Stage 2 — permissions

Delete every `Resource*` constant that names a table and every `Requires("…")`/`Permission: "…"`
literal: a table's permission is `model.ResourceOf(&XModel)` (same name as today when the constant
already equalled the table name). A permission that is not a table is declared once:
`var ResourceX = model.FeatureResource("x")`. Say in the PR any name that changes.

## Stage 3 — save and delete of the screens

- `OpUpsertItem`: `view.MountSave(reg, OpUpsertItem, func(i *SupplyItem) error { out, err := m.SaveItem(*i); if err == nil { *i = out }; return err }).Requires(model.ResourceOf(&SupplyItemModel), model.Create|model.Update)`.
- `OpUpsertLocation`: `view.MountSave(reg, OpUpsertLocation, func(l *SupplyLocation) error { out, err := m.SaveLocation(*l); if err == nil { *l = out }; return err }).Requires(model.ResourceOf(&SupplyLocationModel), model.Create|model.Update)`.
- `OpDeleteItem`: `view.MountDelete(reg, OpDeleteItem, m.DeleteItems).Requires(model.ResourceOf(&SupplyItemModel), model.Delete)`.
- `OpDeleteLocation`: `view.MountDelete(reg, OpDeleteLocation, m.DeleteLocations).Requires(model.ResourceOf(&SupplyLocationModel), model.Delete)`.
- Delete the replaced handlers and every args type that only re-declared `{records}`/`{ids}` (regenerate `model_orm.go` if a definition goes away).

## Stage 4 — records and foreign keys

Declare `Ref: &XModel, DB: &model.FieldDB{RefColumn: "id"}` on every `*_id` column that points to a
table of this module (keep `PK: true` where the column is part of the key), as
`veltylabs/modules/agreement_manager/model.go:68` does; regenerate `model_orm.go` with the repo's
`ormc`. A column pointing to another module's table gets `Ref` only if `go.mod` already requires
that module.

## Stage 5 — translations

English keys only (grep `ui/` and the root for Spanish literals used as `lang.Text`, labels or
input options; make them English keys). Fill `lang.json` with **real** translations for every
language the file declares — never empty, never a placeholder. `langc sync .` → 0 pending, 0 reuse
warnings, 0 identical values.

## Stage 6 — tests and demo

- Update the tests that used deleted types or handlers.
- New `tests/screen_save_test.go`: through `view.NewRecords().Caller(loopback.ActingAs(...))` and `view.NewCallerLister` with each presenter's `view.Ops`, save and delete one record per screen (`OpUpsertItem`, `OpUpsertLocation`, `OpDeleteItem`, `OpDeleteLocation`): no error, row stored then gone; one duplicate/conflict case answers `model.KindOf(err) == model.ErrorConflict` on the client.
- The demo opens with `webtyp` at the repo root; attach a screenshot of each screen to the PR.

## Stage 7 — docs (Spanish)

`README.md`: errores por `rpc.WriteError`; permisos por tabla (`model.ResourceOf`); guardar/borrar con `view.MountSave`/`MountDelete`; claves
foráneas declaradas.

## Delivery check

```
GOPRIVATE=webtyp.com/*,github.com/veltylabs/* GOFLAGS=-mod=mod gotest
```

- A PR that does not compile, or whose tests could not run, is a failed delivery.
- Never pause to ask permission: when the check passes, commit, push and open the PR. Stuck 30
  minutes: open the PR listing what fails.
- `git diff --stat main` lists only files of this repo touched by the stages, `go.mod`, `go.sum`,
  this plan. No stray scripts. No client domain in any example.

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | errors, handlers | compiles |
| 2 | ops, `ui/` | compiles |
| 3 | ops | compiles |
| 4 | `model.go`, `model_orm.go` | compiles |
| 5 | `lang.json`, `ui/`, `model.go` | `langc sync .` clean |
| 6 | `tests/*`, demo | suite green, screenshots in PR |
| 7 | `README.md` | text present |
