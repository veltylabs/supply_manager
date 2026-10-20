# Architecture

## Scope

`supply_manager` holds the clinic's clinical supplies (gloves, syringes, gauze…): what exists, the batches they come in (a supply may have no lot), where they are kept and every movement of stock. The pharmacy is a different module (`pharmacy_manager`); they share the rules in the library `webtyp.com/stock` but do not share tables or code, and neither imports the other.

There is no stock screen in this version.

## Entities and Rules

- **All stock is held per batch.** The rules of stock (first expiry first out, expired lots, never negative, movement directions) are those of `webtyp.com/stock`; nothing in this module repeats them.
- **A movement is append-only**: there is no update and no delete; a mistake is corrected by a new movement. Its `quantity` is always greater than zero; the direction comes from the movement type.
- **Every stock change is one transaction** with an optimistic revision check on the stock level (`ErrConflict` when the row changed).
- **Cost**: a batch has an optional `unit_cost` (whole pesos per unit, 0 = unknown). With `Deps.Publisher` and `Deps.MoneyScope` set, each output movement whose batch has a cost publishes one `money.Consumed` fact (cost × quantity, rounded half away from zero). A purchase is never a fact of this module.
- **Events**: one `supply.movement.registered` per movement, after the commit.
- **Tenancy**: every query of a stock operation filters by `tenant_id`; the operations the generic view drives resolve the row by id and use the tenant the row stores.
- Units: `PIECE`, `BOX`, `PAIR`, `ML`, `GRAM`. Supplies have **no price, no tax treatment and no sale flag**.
- A batch's expiry is **optional** (0 = does not expire); a supply with no lot uses the batch number `NoBatchNumber` (`NO-BATCH`), passed explicitly.
- Outputs accepted: `stock.InternalUse` (the use in a consultation) and `stock.AdjustOut` (a write-off); `stock.Sale` is `ErrMovementType`. An expired batch is never used (`ErrBatchExpired`); only a write-off may take expired stock. `min_quantity` (0 = no alert) feeds `ListLowStock`.


- **Rooms**: a location may name the room (box) it supplies (`room_id`, a soft reference to `room_layout`); at most one
  active location per room and tenant (`ErrRoomTaken`). `UseInRoom` consumes from that location; the room picker in the
  location form comes with the stock screens.

## Dependencies

This module depends on:
- `webtyp.com/stock` - Rules for stock are defined here (FEFO, expiry, no negative stock).
- `webtyp.com/money` - Used for recording `money.Consumed` facts on output movements.

## Composition Root Example

```go
m, err := supplymanager.New(db, supplymanager.Deps{
	IDs:        ids,
	Publisher:  broker,
	MoneyScope: "supply",
})
```

## Generic View Operations

Why `tenant_id` is absent from the operations the generic view drives: they resolve the row by id and use the tenant the row stores.
