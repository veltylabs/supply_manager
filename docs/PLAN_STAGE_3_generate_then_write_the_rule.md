← [Stage 2](PLAN_STAGE_2_module_go.md) | Next → [Stage 4](PLAN_STAGE_4_readers_used_by_other_module.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 3 — generate, then write the rules

1. Delete `ops.go`, `crud.go`, `queries.go`, `model_crud.go`, `model_orm.go` and `definition_crud.go` (those that exist). Run `definitionc`: it writes one `definition_gen.go` (the types, the schemas and, on `Module`, `ModelName()`, `Definition()`, `MountOperations()`) and the skeleton `rule_<entity>_<verb>.go` of every action declared `AddMyBusinessRule`.
2. Add `store.go`: for each table of «Tables, stores and rules» that has a Store, `// Store<Entity> is the store of <entity>.` `func (m *Module) Store<Entity>() crud.Store { return m.store }` (the module's global store for a table the plan lists with `orm.GlobalStore`).
3. Fill each skeleton with the body of the rule named in «Tables, stores and rules» (and «Read models»), in the forms of «The rule of this plan». An old rule `func(c crud.Caller, tx, rec) error` becomes the rule method with the same body, `c.UserID` becomes `a.UserID`, a nil error becomes `return crud.ApplyAndPublish, nil`, an error `e` becomes `return 0, e`.
4. Specific to this module: Every rule named above is a rule file. `runOperation(c, tx, op)` switches on `op.Kind` (unknown kind → field error on `kind`) and runs the moved logic with `tx`; `webtyp.com/stock` stays the only place with allocation arithmetic.
Delete every `Op*` constant, `res*`/`Resource*` variable, every `op*` handler, every `view.MountSave`/`view.MountDelete`/`reg.Operation` call and the `webtyp.com/router`, `webtyp.com/rpc` imports that only they used (`grep -rn "reg.Operation\|view.MountSave\|view.MountDelete\|router.Context\|rpc.WriteError" --include=*.go . | grep -v _test` → empty).
