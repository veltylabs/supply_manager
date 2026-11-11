← [Stage 5](PLAN_STAGE_5_browser_side.md) | Next → [Stage 7](PLAN_STAGE_7_translations_and_documentati.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 6 — tests (`tests/`, through `crud.LoopbackActingAs(nil, "u1", mod)`; rows in `docs/TESTS.md`)

Delete tests of removed code. Operations are `router.Operation{Module: ModelName, Entity: "<entity>", Action: model.<Action>}`, never a string; fixtures follow `crud` v0.0.10 (`reg.AddStoredEntity`, `crud.CurrentUser`). Add `TestMount_MatchesDefinition` (`crud.Mount(reg, nil, nil, mod)` returns nil) and one test that an allowed write publishes a `*crud.Change` on `crud.Topic(ModelName, "<entity>", model.<Action>)`. Required:

**How a test creates and reads (exact; copy it, never call `m.CreateItem`-style methods that no longer exist):**

```go
func opOf(entity string, a model.Action) router.Operation {
	return router.Operation{Module: supplymanager.ModelName, Entity: entity, Action: a}
}

// create one row
var saved supplymanager.SupplyItem
var err error
caller := crud.LoopbackActingAs(nil, "u1", mod)
caller.Call(opOf("supply_item", model.Create),
	&crud.CreateArgs{Records: []model.Model{&supplymanager.SupplyItem{Id: "i1", Name: "Gauze"}}}, &saved, func(e error) { err = e })

// read one row by id, or all rows (a list is the generated <Entity>List type)
var one supplymanager.SupplyItemList
caller.Call(opOf("supply_item", model.Read), &crud.ReadArgs{ID: "i1"}, &one, func(e error) { err = e })
var all supplymanager.SupplyItemList
caller.Call(opOf("supply_item", model.Read), &crud.ReadArgs{Limit: 100}, &all, func(e error) { err = e })
```

Filtering is a field of the read model or table (`crud.ReadArgs{Filter: &supplymanager.SupplyItem{LocationId: "l1"}}` for a table; the fields of the read model for `ExpiringStock`/`LowStock`); paging is `ReadArgs{After, Limit}`. There is no `model.List` and no `ListItems`: the read is always `model.Read` through the caller. The test helper `mod, db := setup(t)` builds `orm.New(mem.New())`, runs `migrate.Migrate`, and calls `New(db, Deps{TenantID: "t1", IDs: <fake id generator>})`.

| Test | Setup → call → expected |
|---|---|
| `TestTenant_ReadOtherTenantIsNotFound` | product/item of tenant a read by tenant b → `model.ErrorNotFound` |
| `TestOperation_ReceiveCreatesBatchAndLevel` | create operation `receive` → one batch, stock level = quantity, one movement; operation `movement_ids` lists it |
| `TestOperation_OutputFirstExpiryFirst` | two batches expiring day 5 and day 2; output 3 → taken from the day-2 batch first |
| `TestOperation_OutputNeverNegative` | output more than stock → error, no movement, levels unchanged (atomic) |
| `TestOperation_TransferMovesBetweenLocations` | transfer 2 from A to B → level A −2, level B +2, two movements |
| `TestOperation_UnknownKindRejected` | kind "x" → field error on `kind` |
| `TestMovement_NoWriteOperations` | create on `supply_movement` through the loopback → a non-nil error (the entity is read-only) |
| `TestReadModel_LowStock` | item under its minimum → appears in `LowStock` |
| `TestDemo_Valid` (`//go:build wasm`) | `seed.Demo()` → no error, `Validate()` nil, at least one screen |
