← [Stage 3](PLAN_STAGE_3_generate_then_write_the_rule.md) | Next → [Stage 5](PLAN_STAGE_5_browser_side.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 4 — readers used by other modules

Signatures of every exported reader stay (other modules and apps call them). Bodies read through `m.store` (`ReadOne`, `ReadAll` with `crud.Query`), never `m.db` directly; a `tenantID` parameter different from the module's tenant answers «not found» (`found=false` / empty, `err=nil`). Keep `ItemExists`, `ItemCard`, `GetItem`, `GetLocation`, `GetBatch`, `ListItems`, `ListLocations`, `ListBatches`, `ListStock`, `ListMovements`, `ListExpiring`, `ListLowStock`, `ReviewExpiring`, `UseInRoom`, `ReceiveStock`, `RegisterEntry`, `RegisterOutput`, `TransferStock` (each builds the `SupplyOperation` record and runs `m.runOperation` inside `m.store.Atomic`).
