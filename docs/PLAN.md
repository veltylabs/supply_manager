---
PLAN: "feat: supply_manager v0.1.0 — the clinic supplies: items, batches with optional expiry, locations and append-only stock movements, FEFO output over webtyp/stock, cost facts over webtyp/money, UI and demo"
TAG: v0.1.0
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 11293930010196048715
---

> Read AGENTS.md at the repo root before any change.
> This is a **new module**. `gonew` created only `go.mod`, `LICENSE`, `README.md` and a stub `supply_manager.go`.
> The reference implementation to replicate, file by file, is
> [`github.com/veltylabs/item_catalog`](https://github.com/veltylabs/item_catalog) (read its `model.go`, `mcp.go`,
> `view.go`, `ui/`, `seed/`, `web/client.go`, `migrate/` and `tests/` before writing anything). Where this plan
> and item_catalog differ, this plan wins; where this plan is silent, do what item_catalog does.
> Dependencies that are **libraries** (`webtyp.com/stock`, `webtyp.com/money`, `webtyp.com/events`) are published; use their
> exported API exactly as documented in their README and doc comments. If any symbol this plan names is missing from the
> published version, **stop** and say which in the PR.

## Tools

```
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/ormc/cmd/ormc@latest
go install webtyp.com/lang/cmd/langc@latest
```

## What this module is (and is not)

The clinic uses clinical **supplies** (gloves, syringes, gauze, dressings…) in its consultations and procedures. Today
they are not tracked. This module is the system of record for **what the clinic has of them**:

- a supply **item**,
- the **batches** it comes in (a supply may have **no lot**: it then uses the batch number `NoBatchNumber`; expiry optional;
  optional unit cost),
- the **locations** it is kept in (storeroom, a consulting room),
- every **movement** of stock (append-only).

The **pharmacy** is a different module, `pharmacy_manager`, with different managers, permissions and rules; nothing here
knows it. Supplies are **not sold and have no price** here. A supply used in a consultation is recorded by
`clinical_encounter` (a later plan) as a `stock.InternalUse` output that carries the encounter id
(`source_document_type` / `source_document_id`, free text this module never interprets). The stock is deducted **once,
at use**. This module is not a billing or accounting module and knows nothing about patients or services.

## Ecosystem search (done before writing this plan)

| Found | What it offers | Decision |
|---|---|---|
| `webtyp/stock` v0.1.0 | `Movement` (+ `Direction`), `Lot`, `Allocate` (first expiry first out, all-or-nothing), `Apply` (never-negative arithmetic), `Lot.Expired`, `Lot.ExpiresWithin`, and `stocktest.Run` (conformance suite) | **use for every stock rule**. This module writes no FEFO, no movement-type table and no expiry predicate of its own |
| `webtyp/money` v0.1.0 | `Money`, `Currency` (`CLP`), `Fact` (`Consumed`), `Publish(events.Publisher, Fact)` | **use** for the cost fact of an output (see «Cost»). No local money type |
| `webtyp/events` v0.1.0 | `Publisher`, `Event`, `model.Encodable` payloads | **use** (`Deps.Publisher`, optional) |
| `veltylabs/pharmacy_manager` (sibling module) | the same stock concerns for a different area | **never imported** (modules do not import each other). The rules they share are in `webtyp/stock`; the tables are separate on purpose (`ormc` generates each table from a static literal inside its own module) |
| `veltylabs/item_catalog` | module shape: model, ops, view, ui, seed, web, migrate, tests | **replicate** |
| `veltylabs/appointment_booking` | optimistic concurrency with `revision` inside a transaction (`repository.go`, `UpdateReservationStatusTx`) | **replicate** the pattern; read it first |
| `webtyp/notify`, `webtyp/bus` | alerts / a retired pub/sub | not used here: a later module (`alert_manager`) reads `list_low_stock` and `list_expiring` and raises the alerts |

## Design gate (public API of a new module)

1. **Prior art:** `item_catalog` (module shape); the clinic's previous system (`inventory_location`, `inventory_batch`,
   `inventory_stock`, `inventory_movement`; lessons its own reviews recorded: the quantity is absolute and the sign comes
   from the movement type; stock is deducted once, at use); Odoo's and ERPNext's stock ledger (append-only moves, on-hand
   as a sum, cost per lot).
2. **Novice-name test:** `Item`, `Location`, `Batch`, `StockLevel`, `Movement`, `ReceiveStock`, `RegisterOutput`,
   `TransferStock`, `ListExpiring`, `ListLowStock`. A reader guesses each.
3. **Complexity ledger:** + 5 entities, + 19 operations, + 2 readers. Nothing deleted (new module).
4. **Where it belongs:** its own repo; it depends only on webtyp libraries, never on another veltylabs module.
5. **What it deletes:** nothing.

## Constants, vocabulary and rules (the contract; implement exactly this)

Package name `supplymanager`. `ModelName = "supply"` (wire and RBAC prefix; `ui.ID = "supply"`).

**Units** (`Unit*`, closed set): `UnitPiece = "PIECE"`, `UnitBox = "BOX"`, `UnitPair = "PAIR"`, `UnitMl = "ML"`, `UnitGram = "GRAM"`.

**Movement types** are `webtyp.com/stock`'s `stock.Movement` (`stock.Purchase`, `stock.AdjustIn`, `stock.TransferIn`,
`stock.AdjustOut`, `stock.Sale`, `stock.InternalUse`, `stock.TransferOut`) and their direction is `Movement.Direction()`.
The stored `quantity` is always **> 0**; **no local constant duplicates them**. Outputs this module accepts through
`RegisterOutput`: `stock.InternalUse`, `stock.AdjustOut` (there is no sale).

### Entities (`model.Definition` literals in `model.go`; every one has `id` (PK) and `tenant_id` (NotNull))

Table names are prefixed `supply_` so that this module and its sibling never collide in one database.

**`supply_item`** (`ItemModel`)

| Field | Type / widget | Rules |
|---|---|---|
| `sku` | `input.Text()`, Label "SKU" | NotNull; letters, digits and `-`; 1–50; unique per tenant → `ErrSkuExists` |
| `name` | `input.Text()`, Label "Name" | NotNull; 1–255 |
| `unit` | `input.Select()` of `Unit*`, Label "Unit" | NotNull; closed set |
| `min_quantity` | `input.Number()`, `model.Float()`, Label "Minimum quantity" | `>= 0`; `0` = no low-stock alert |
| `is_active` | `input.Checkbox()`, Label "Active" | NotNull |
| `updated_at` | `model.Int()`, `OmitEmpty` | |

A supply has **no price, no tax treatment and no sale flag**: it is not sold.

**`supply_location`** (`LocationModel`): `name` (`input.Text()`, Label "Name", 1–100, unique per tenant →
`ErrLocationNameExists`), `is_active` (`input.Checkbox()`, Label "Active"), `updated_at`.

**`supply_batch`** (`BatchModel`): `item_id` (Ref → `ItemModel`, NotNull), `batch_number` (Text, 1–50, unique per
item → `ErrBatchExists`), `expiration_date` (Int, Unix seconds UTC; `0` = no expiry), `unit_cost` (Int, **whole Chilean
pesos per unit**, `0` = unknown, never negative), `is_active`, `updated_at`.
`NoBatchNumber = "NO-BATCH"` is the batch number of a supply that has no lot; expiry is optional (`0` = none).

**`supply_stock_level`** (`StockLevelModel`): `location_id` (Ref, NotNull), `batch_id` (Ref, NotNull), `quantity` (Float),
`revision` (Int), `updated_at`. One row per (location, batch), unique. `quantity` is never negative (`stock.Apply`).

**`supply_movement`** (`MovementModel`): `batch_id`, `location_id` (Refs), `movement_type` (`stock.Movement`), `quantity`
(Float, **> 0**), `source_document_type` (Text, optional), `source_document_id` (Text, optional), `note` (Text,
optional, ≤ 255), `created_at` (Int, Unix seconds UTC), `created_by` (Text, optional: the user id from the caller's
context if the router exposes it; otherwise empty). **Append-only: no update, no delete, no operation for either.**
Correcting a mistake is a new movement.

### Errors (`domainError` constants, like item_catalog; status in `statusOf`)

404: `ErrNotFound` ("supply: not found"), `ErrItemNotFound`, `ErrLocationNotFound`, `ErrBatchNotFound`.
409: `ErrSkuExists`, `ErrLocationNameExists`, `ErrBatchExists`, `ErrConflict` ("supply: concurrent update, retry"),
`ErrBatchExpiryMismatch` (receiving into an existing batch number with a different expiry), `ErrBatchCostMismatch`
(receiving into an existing batch with a different non-zero unit cost).
400: `ErrInsufficientStock`, `ErrBatchExpired`, `ErrItemInUse` (delete a item that has batches),
`ErrLocationInUse` (delete a location that has stock rows), `ErrSameLocation`, `ErrItemInactive`, `ErrLocationInactive`,
`ErrNoRecords`, `ErrNoIds`, plus `ValidationError` and `decodeError` as in item_catalog. Map `stock.ErrInsufficient` to
`ErrInsufficientStock` and `stock.ErrQuantity` to `ValidationError`. Every message is a constant string prefixed
`supply: `; never `errors` from the stdlib (`fmt.Err` from `webtyp.com/fmt`).

### Operations (names are constants `Op*`; mount in `MountOperations` with `Requires(resource, action)`)

The resource of an operation is the table it works on (`supply_item`, `supply_location`, `supply_batch`, `supply_stock_level`,
`supply_movement`) so that a role can be granted exactly one of them.

| Constant = wire name | Resource / action | Does |
|---|---|---|
| `OpListItems = "list_items"` | `supply_item` / Read | filter by `active_only`, `limit`, `offset` |
| `OpGetItem = "get_item"` | `supply_item` / Read | |
| `OpCreateItem = "create_item"`, `OpUpdateItem = "update_item"`, `OpUpsertItem = "upsert_item"` | `supply_item` / Create, Update, Create\|Update | per item_catalog's `CreateItem`/`UpdateItem`/`UpsertItem` |
| `OpDeactivateItem = "deactivate_item"`, `OpDeleteItem = "delete_item"` | `supply_item` / Update, Delete | delete only if the item has no batch |
| `OpListLocations = "list_locations"`, `OpUpsertLocation = "upsert_location"`, `OpDeleteLocation = "delete_location"` | `supply_location` | delete only if no stock-level row references it |
| `OpListBatches = "list_batches"` | `supply_batch` / Read | by `item_id` (required) |
| `OpReceiveStock = "receive_stock"` | `supply_movement` / Create | see below |
| `OpRegisterEntry = "register_entry"` | `supply_movement` / Create | see below |
| `OpRegisterOutput = "register_output"` | `supply_movement` / Create | see below |
| `OpTransferStock = "transfer_stock"` | `supply_movement` / Create | see below |
| `OpListStock = "list_stock"` | `supply_stock_level` / Read | by optional `item_id`, `location_id`; only rows with `quantity > 0` |
| `OpListMovements = "list_movements"` | `supply_movement` / Read | by optional `item_id`, `batch_id`, `from`, `to`, `limit` (default 100), newest first |
| `OpListExpiring = "list_expiring"` | `supply_stock_level` / Read | see below |
| `OpListLowStock = "list_low_stock"` | `supply_stock_level` / Read | see below |

Each op has its own `…Args` model (first field `tenant_id`), like item_catalog's `ListItemsArgs`.

**`ReceiveStock{ItemId, LocationId, BatchNumber, ExpirationDate, UnitCost, Quantity, SourceDocumentType, SourceDocumentId, Note}`**
Finds the batch of that item with that `BatchNumber` or creates it (applying the batch rules; if it exists and its
expiry differs → `ErrBatchExpiryMismatch`; if it exists and both costs are non-zero and differ → `ErrBatchCostMismatch`;
a `UnitCost` of 0 into an existing batch keeps the stored cost), then adds `Quantity` (> 0) to the (location, batch)
stock level (creating it at 0 if missing, using `stock.Apply`) and appends a `stock.Purchase` movement. The item and the
location must exist in the tenant and be active (`ErrItemInactive`, `ErrLocationInactive`). Returns the movement.

**`RegisterOutput{ItemId, LocationId, Quantity, MovementType, BatchId, SourceDocumentType, SourceDocumentId, Note}`**
`MovementType` ∈ {`stock.InternalUse`, `stock.AdjustOut`}; any other value (including `stock.Sale`) → `ValidationError`.
An expired batch is never used (`ErrBatchExpired`); only `AdjustOut` may take expired stock.
- `BatchId` empty → **first expiry first out** through `stock.Allocate(lots, quantity, now)`: build the `[]stock.Lot` from
  the stock levels of that item in that location (`Lot{ID: batch id, Expiry: expiration_date, Quantity: level quantity}`),
  take what `Allocate` returns, one movement per `stock.Take`. `stock.ErrInsufficient` → `ErrInsufficientStock` and
  **nothing is written**. Do not re-implement ordering, expiry or tolerance.
- `BatchId` given → only that batch. If `Lot.Expired(now)` and the type is not `stock.AdjustOut` → `ErrBatchExpired`
  (writing expired stock off is exactly what `AdjustOut` is for). If its quantity is smaller → `ErrInsufficientStock`.
- Returns the list of movements.

**`RegisterEntry`** — an **inbound correction** (a count found more units than the book): `{ItemId, LocationId, BatchId,
Quantity, Note}` adds to an **existing** batch and appends a `stock.AdjustIn` movement.

**`TransferStock{ItemId, FromLocationId, ToLocationId, Quantity, Note}`** — same location → `ErrSameLocation`. Takes
`Quantity` out of the source with `stock.Allocate` (`stock.TransferOut` movements) and puts the same batches into the
destination (`stock.TransferIn`, same quantities), **atomically**.

**`ListExpiring{WithinDays}`** — stock levels with `quantity > 0` whose batch satisfies `Lot.ExpiresWithin(now,
WithinDays)` (expired ones included), as an `ExpiringStock` list `{item_id, item_name, batch_id, batch_number,
location_id, quantity, expiration_date, expired bool}` ordered by `expiration_date` ascending. `WithinDays < 0` →
`ValidationError`.

**`ListLowStock{}`** — the items that are active, have `min_quantity > 0` and whose **total quantity over all locations
and batches** is `<= min_quantity`, as `LowStock{item_id, item_name, total, min_quantity}` ordered by name. A later
`alert_manager` calls it; this module raises no alert itself.

**Atomicity and concurrency.** Each of `ReceiveStock`, `RegisterOutput`, `RegisterEntry` and `TransferStock` writes its
stock-level changes and its movements in **one database transaction**. Every stock-level update is conditional on the
`revision` read (`WHERE id = ? AND tenant_id = ? AND revision = ?`, then `revision + 1`); a lost race returns
`ErrConflict`. Copy the transaction/revision pattern of `appointment_booking` (`repository.go`,
`UpdateReservationStatusTx`; its `AGENTS.md`, «Concurrencia optimista»). Read it first.

**Multi-tenancy.** Every query and every write carries `tenant_id` in the condition; an id of another tenant is
`ErrNotFound`. No operation takes a tenant from anywhere but its args (as item_catalog does).

**Events** (`Deps.Publisher` optional; nil disables): `TopicMovementRegistered = "supply.movement.registered"`
(payload: the movement), published **after** commit, once per movement. No other topic of its own.

**Cost (accounting-ready).** An output whose batch has `unit_cost > 0` consumes goods at cost. When `Deps.MoneyScope` is
non-empty (the app names the area of the business the money belongs to, e.g. `"supply"`; empty disables) and
`Deps.Publisher` is non-nil, after commit publish through `money.Publish(Deps.Publisher, fact)` one fact per output
movement (`Sale`, `InternalUse`, `AdjustOut`; not entries, purchases or transfers):

```go
money.Fact{
	Kind:   money.Consumed,
	Side:   money.Outflow,
	Amount: money.Money{Amount: roundedCost, Currency: money.CLP}, // unit_cost × quantity, half away from zero; skip when it rounds to 0
	Scope:  deps.MoneyScope,
	Source: money.Ref{Type: "supply_movement", ID: movementID},
	At:     createdAt,
}
```

Use `money.New` to build the `Money` and return its error if any. A publish error is **returned** to the caller after
the commit (the stock change is already stored; say so in the error text), never swallowed. Purchases are **not** a fact
of this module (a later `purchase_manager` owns them): never publish `money.Purchased` here.

**Cross-module readers** (other modules declare matching interfaces; this module's `*Module` satisfies them
structurally; do not import them): `ItemExists(tenantId, itemId string) (bool, error)` — exists and active;
`ItemCard(tenantId, itemId string) (name, unit string, active bool, err error)` — absent or foreign tenant ⇒ `ErrNotFound`.

## Stage 1 — scaffold

1. Delete the stub `supply_manager.go`.
2. Create `model.go` (the `Definition` literals, the constants, the errors, the topic, `ModelName`), run `ormc` to
   generate `model_orm.go` (never edit it), `go mod tidy`.
3. Dependencies: the same `webtyp.com/*` set as `item_catalog/go.mod` (`fmt`, `model`, `input`, `orm`, `router`, `view`,
   `events`, `ddl`, `time`, `unixid`, `lang`, and for `ui/` and `web/`: `dom`, `layout`, `svg`, `components`, `storage`)
   **plus** `webtyp.com/stock` v0.1.0 and `webtyp.com/money` v0.1.0 (and `webtyp.com/events` at the version
   `go get webtyp.com/events@latest` resolves, which must be v0.1.0 or higher). Use the versions item_catalog's `go.mod`
   pins for the rest; **no local `replace`**.
4. `migrate/migrate.go`: `Migrate(conn ddl.Execer, c ddl.Compiler) error` creating the five tables in FK order
   (`Item`, `Location`, `Batch`, `StockLevel`, `Movement`), like item_catalog's.

## Stage 2 — the module

`module.go`: `Deps{IDs model.IDGenerator (required), Publisher events.Publisher (optional), MoneyScope string (optional)}`,
`Module`, `New` (error text `"supply_manager: Deps.IDs is required"`), the service methods named in the operations table,
the two readers. `mcp.go`/`ops.go`: `ModelName()`, `MountOperations`, the handlers and `writeError`/`statusOf`,
exactly as item_catalog's. `interfaces.go` is **not** needed (this module defines no port).

Time: use `webtyp.com/time` for "now" (Unix seconds). Do **not** add a clock parameter, option or seam for tests: tests
use expiry dates far in the future and in the past.

## Stage 3 — view

`view.go`: `Item()` for `Item` (`Label` = name, `Description` = sku) and `Location` (`Label` = name);
`NewItemView(caller) view.Presenter`, `NewLocationView(caller) view.Presenter`, built like item_catalog's `NewView`
(`view.NewCallerLister` + `view.New`), with titles "Items" and "Locations" as named constants.

## Stage 4 — `ui/`, `seed/`, `web/`

- `ui/module.go`: `ID = "supply"`, `Label = "Insumos"`, icons `IconSupply = svg.Icon("supply")`,
  `IconItems = svg.Icon("supply_items")`, `IconLocations = svg.Icon("supply_locations")`.
- `ui/svg.go` (`//go:build !wasm`): one sprite entry per icon, all three with the same simple box geometry
  (`viewBox "0 0 16 16"`, one path): `M2 4l6-3 6 3v8l-6 3-6-3V4zm6-1.7L4.2 4.2 8 6.1l3.8-1.9L8 2.3zM3 5.4v6l4.5 2.2V7.6L3 5.4zm10 0L8.5 7.6v6L13 11.4v-6z`.
  Do not draw other artwork.
- `ui/browser.go`: `Browser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error)`
  returns `platformd.NewGroupUIModule(ID, Label, IconSupply, children)` with two children built as in
  `item_catalog/ui/browser.go`: `ID+".items"` ("Artículos", a `crudview` over `NewItemView`) and
  `ID+".locations"` ("Ubicaciones", a `crudview` over `NewLocationView`). `NewRecord` for a item: `Unit: UnitPiece, IsActive: true`;
  for a location: `IsActive: true`. **No screen for stock, batches or movements in this version** (a later plan designs the
  entry/exit forms); say so in `docs/ARCHITECTURE.md`.
- `seed/seed.go`: `Load(m *Module, tenantID string) (Data, error)` through the module's own methods: two locations ("Bodega" and "Box 1"), three items (gloves in `UnitPair`, syringes in `UnitPiece`, gauze in `UnitBox`),
a batch received for each: the gloves with no lot (`NoBatchNumber`), the syringes with a future expiry and a unit cost, the
gauze with a lot and no expiry.
- `web/client.go` (`//go:build wasm`): the runnable in-browser demo, copy of item_catalog's with `supplymanager`
  (`storage/mem`, `router/loopback`, the seed, `Browser`).
- All user-visible text in `ui/` and `view.go` is `lang.Text` with English keys; run `langc sync` and add the Spanish
  translations in `lang.json` (Insumos, Artículos, Ubicaciones, Nombre, Unidad, Par, Cantidad mínima, Activo, and every error message that reaches the person). The Spanish is
  gender-neutral (skill **translations**).

## Stage 5 — tests (`tests/`, external package `supplymanager_test` unless noted, `orm.New(mem.New())`, no mocks of the module)

Behaviour and rules only, never the look. Group them in files by feature; do not duplicate a rule in two files. Every
test file that imports `webtyp.com/stock/stocktest` carries `//go:build !wasm`.

1. **Conformance of the shared rules:** a small `ledger_test.go` adapter implements `stocktest.Ledger` on top of this
   module (it creates the item and the location on first use and maps the suite's ids to them) and calls
   `stocktest.Run(t, newLedger)`. This is the proof that this module obeys the same stock rules as its sibling. Do not
   copy the suite's cases into other tests.
2. **Items:** create/get/update/deactivate/delete; `ErrSkuExists` (per tenant, not across tenants); 
   delete with a batch → `ErrItemInUse`; foreign tenant → `ErrNotFound`.
3. **Locations:** unique name, delete in use.
4. **Batches and receiving:** `NoBatchNumber` is accepted and a blank batch number is `ValidationError`; receiving twice into the same batch adds up; different expiry →
   `ErrBatchExpiryMismatch`; different non-zero cost → `ErrBatchCostMismatch`; cost 0 into an existing batch keeps it.
5. **Output:** the `InternalUse` and `AdjustOut` types work and others are `ValidationError`; a failed output leaves stock and
   movements unchanged; forced `BatchId` expired → `ErrBatchExpired` unless `AdjustOut`; `Sale` is refused with `ValidationError`; a batch with no expiry is drawn last by FEFO.
6. **Transfer:** moves the same batches, `ErrSameLocation`, insufficient stock leaves both locations unchanged.
7. **Invariants:** quantity stored is always > 0; stock never negative after any sequence; the sum of `list_stock` equals
   the sum of in-movements minus out-movements for a item (a property test over a short scripted sequence).
8. **Concurrency:** a stale `revision` on a stock-level update returns `ErrConflict`. The public API cannot force a stale
   revision, so this one test is a **root-level test** (`conflict_internal_test.go`, package `supplymanager`) whose first line is
   `// Root-level test (justified): exercises the unexported stock-level update — the public API cannot force a stale revision.`
9. **`ListExpiring` and `ListLowStock`:** expiring includes expired and soon-to-expire, excludes far-future and empty
   rows; negative days → error; low stock includes the item at exactly `min_quantity`, excludes `min_quantity == 0` and
   inactive ones, and sums across locations and batches.
10. **Events and cost:** one `supply.movement.registered` per movement, none when `Publisher` is nil, none on a failed
    operation; with `MoneyScope` set and a batch cost, each output publishes one `money.Consumed` fact with the right
    amount (check a fractional quantity rounds half away from zero), a purchase publishes none, no `MoneyScope` →
    none, cost 0 → none. Use `events/mock`'s broker and `money.Subscribe` with a recording `money.Sink`.
11. **Conformance of the module:** it satisfies `router.OperationModule`; every op is mounted with `Requires` and
    `Accepts` (copy item_catalog's `conformance_test.go`).
12. **View test** (`tests/supply_view_test.go` + wasm variant as item_catalog's): the group has the two children; the
    item form renders its fields with their labels.

## Stage 6 — docs

`README.md` (quick start, Ops table, readers, the cost fact, key files), `docs/ARCHITECTURE.md` (scope, entities, rules
above, the "no stock screen yet" note, the relation to `webtyp/stock` and `webtyp/money`, composition-root example that
shows `Deps{IDs, Publisher, MoneyScope}`), `docs/diagrams/database.md` (Mermaid ERD of the five tables). Update the
`AGENTS.md` domain notes only if this plan contradicts them. Delete nothing else. `docs/PLAN.md` itself is deleted by
the workflow, never by you.

## Delivery check

```
gotest                                                  # vet, race, tests and wasm green
grep -rn "errors\." --include=*.go . | grep -v _test    # → empty (no stdlib errors)
grep -rn "Purchased" --include=*.go . | grep -v _test   # → empty (purchases are not a fact of this module)
grep -n '^replace' go.mod                               # → empty
git diff --stat main                                    # only the files named in the stages
```

## Stages

| # | Stage | Files |
|---|---|---|
| 1 | scaffold | `model.go`, `model_orm.go` (generated), `go.mod`, `migrate/migrate.go` |
| 2 | the module | `module.go`, `mcp.go`/`ops.go` |
| 3 | view | `view.go` |
| 4 | ui, seed, demo | `ui/*`, `seed/seed.go`, `web/client.go`, `lang.json` |
| 5 | tests | `tests/*` |
| 6 | docs | `README.md`, `docs/ARCHITECTURE.md`, `docs/diagrams/database.md` |
