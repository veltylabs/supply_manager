← [Stage 1](PLAN_STAGE_1_definition_go.md) | Next → [Stage 3](PLAN_STAGE_3_generate_then_write_the_rule.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 2 — `module.go`

`Deps` keeps its fields; add `IDs model.IDGenerator` (required, `depErr("IDs")` when nil) if it is missing.
`Module` gains `store *orm.Store`, built once in `New`: `m.store = orm.TenantStore(db, deps.TenantID, deps.IDs)`.
Delete the hand-written persistence (create/update/upsert/patch/delete/list/get bodies that only move rows) once its rules moved to the rule files: the row-moving bodies of every create/update/upsert/patch/deactivate/delete/list/get of products/items and locations; `ReceiveStock`, `RegisterEntry`, `RegisterOutput`, `TransferStock` keep their signatures only as readers (below) and call the same rule logic inside `m.store.Atomic`.

## Translation table: old `orm.DB` call → crud call (exact)

The helpers that moved rows (`registerOutput`, `receiveStock`, the batch/level functions…) change the type of their first parameter from `*orm.DB` to `crud.Tx`; a rule passes its `tx`, a kept method passes `m.store` (a `crud.Store` is a `crud.Tx`). One function at a time, `gotest` after each; never `git reset --hard`.

| Old | New |
|---|---|
| `db.Query(&r).Where(X_.Id).Eq(id)` + `ReadOneX(qb, &r)` | `tx.ReadOne(&r, id)`; absent: `model.KindOf(err) == model.ErrorNotFound` |
| `db.Query(&r).Where(X_.A).Eq(v)` + `ReadAllX(qb)` | `tx.ReadAll(func() model.Model { return &X{} }, crud.Query{Conditions: []storage.Condition{storage.Eq("a", v)}, Limit: 500}, func(row model.Model) { rows = append(rows, row.(*X)) })` |
| `db.Create(&r)` | `tx.Create(&r)` |
| `db.Update(&r, orm.Eq(X_.Id, id), …)` | `tx.UpdateFields(&r, id, []string{"col1", "col2"})` (only the listed columns are written) |
| `db.Delete(&X{}, orm.Eq(X_.Id, id))` | `tx.Delete(&X{}, id)` |
| `m.db.Tx(func(tx *orm.DB) error { … })` | `m.store.Atomic(func(tx crud.Tx) error { … })` |

`storage.Eq/Neq/Gt/Gte/Lt/Lte/In/Like` take the snake_case column name as a string (`"item_id"`); a `time.Nano` value goes as `int64(v)`; never a `tenant_id` condition (the store adds it).
