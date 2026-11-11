← [Index](PLAN.md) | Next → [Stage 2](PLAN_STAGE_2_module_go.md)

> Read AGENTS.md and the index `PLAN.md` (rules, tables, external calls) first. Commit when this stage compiles.

## Stage 1 — `definition.go`

- Set `Allow` on each table exactly as in «Tables, stores and rules»; a table not listed there gets no `Allow` (internal).
- Mark `ServerOnly: true`: `tenant_id`, `created_at`, `updated_at`, and every field the server fills (`actor_id`, `movement_ids`, `cost` and every computed amount).
- Delete every `*Args` / `*Result` definition that only carried a removed operation (keep `ReceiveStockArgsModel`, `RegisterEntryArgsModel`, `RegisterOutputArgsModel`, `TransferStockArgsModel` with `Wire: true` — the readers `ReceiveStock`… take them — and `ListExpiringArgsModel` renamed to `ExpiringFilterModel`, `Wire: true`)
  (`grep -n "ArgsModel\|ResultModel" model.go` → only the ones this plan keeps).
- New table `SupplyOperationModel` (name `supply_operation`) with: `id` (PK), `tenant_id`, `kind` (`input.Select()` with the four constants `OperationReceive = "receive"`, `OperationEntry = "entry"`, `OperationOutput = "output"`, `OperationTransfer = "transfer"`), and the union of the fields of `ReceiveStockArgsModel`, `RegisterEntryArgsModel`, `RegisterOutputArgsModel`, `TransferStockArgsModel` (same names and kinds, `OmitEmpty` where only some kinds use them), plus `actor_id`, `movement_ids` (`model.Text()`, comma-joined), `created_at`. Add it to `migrate/` (`Sync`). A field a kind needs and lacks → field error from the rule.
- **Keep every `*Args` model that an exported reader's signature still uses** (`grep -n "^func (m \*Module) [A-Z][A-Za-z]*([^)]*Args" *.go` lists them): mark each `Wire: true`; delete only the `*Args` no remaining signature uses.
- D52 (the entity is its own filter): a filter model (`*Filter`, `*FilterModel`, an `*ArgsModel` that only carried filter values) is not kept: its fields become fields of the read model or table (`OmitEmpty`), and the rule reads them from its receiver; delete the filter type and its `Wire`/`NewFilter` plumbing.
- Add `var Definition = model.ModuleDefinition{Name: "supply_manager", Entities: []model.Definition{…every table and read model of this plan…}}` and, on every table and read model, `Rules` as «Tables, stores and rules» says (`AddMyBusinessRule` where a rule is named, `HaveNoBusinessRule` for the other allowed actions; a read model: `Read: model.AddMyBusinessRule`). Delete the hand topics (`Topic*`) and `Err*`/resources no code uses any more.
