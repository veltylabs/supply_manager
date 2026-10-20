package supplymanager

import (
	"webtyp.com/input"
	"webtyp.com/model"
)

// ModelName is the wire and RBAC prefix of the module: operations are called "<ModelName>.<op>".
const ModelName = "supply"

// Units an item is counted in. The select of the form takes its options from these (ui/browser.go, SetOptions).
const (
	UnitPiece = "PIECE"
	UnitBox   = "BOX"
	UnitPair  = "PAIR"
	UnitMl    = "ML"
	UnitGram  = "GRAM"
)

// NoBatchNumber is the batch number of a supply that has no lot: it is passed explicitly to ReceiveStock.
const NoBatchNumber = "NO-BATCH"

// SupplyItemModel is a clinical supply (gloves, syringes, gauze). It is not sold and has no price here.
var SupplyItemModel = model.Definition{
	Name: "supply_item",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, OmitEmpty: true},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "sku", Type: input.Text(), Label: "SKU", NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'-'}, Minimum: 1, Maximum: 50}},
		{Name: "name", Type: input.Text(), Label: "Name", NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Spaces: true, Tilde: true, Extra: []rune{'.', ',', '(', ')', '-', '/', '%', '+'}, Minimum: 1, Maximum: 255}},
		{Name: "unit", Type: input.Select(), Label: "Unit", NotNull: true},
		{Name: "min_quantity", Type: input.Decimal(), Label: "Minimum quantity", OmitEmpty: true},
		{Name: "is_active", Type: input.Checkbox(), Label: "Active"},
		{Name: "updated_at", Type: model.NanoTime(), OmitEmpty: true},
	},
}

// SupplyLocationModel is a place stock is kept: a storeroom, the counter.
var SupplyLocationModel = model.Definition{
	Name: "supply_location",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, OmitEmpty: true},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "name", Type: input.Text(), Label: "Name", NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Spaces: true, Tilde: true, Minimum: 1, Maximum: 100}},
		// room_id is the room (box) this location supplies, a soft reference to room_layout; "" = none. At most one active
		// location per room. No widget yet: the room picker comes with the stock screens.
		{Name: "room_id", Type: model.Text(), OmitEmpty: true},
		{Name: "is_active", Type: input.Checkbox(), Label: "Active"},
		{Name: "updated_at", Type: model.NanoTime(), OmitEmpty: true},
	},
}

// SupplyBatchModel is one lot of one item (a supply with no lot uses NoBatchNumber). expiration_date is Unix seconds UTC, 0 = does not expire; unit_cost is whole pesos per unit, 0 = unknown.
var SupplyBatchModel = model.Definition{
	Name: "supply_batch",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, OmitEmpty: true},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "item_id", Type: model.Text(), Ref: &SupplyItemModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "batch_number", Type: model.Text(), NotNull: true},
		{Name: "expiration_date", Type: model.UnixTime()},
		{Name: "unit_cost", Type: model.Int(), OmitEmpty: true},
		{Name: "is_active", Type: model.Bool()},
		{Name: "updated_at", Type: model.NanoTime(), OmitEmpty: true},
	},
}

// SupplyStockLevelModel is how much of one batch is in one location. One row per (location, batch); revision guards
// concurrent updates.
var SupplyStockLevelModel = model.Definition{
	Name: "supply_stock_level",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, OmitEmpty: true},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "location_id", Type: model.Text(), Ref: &SupplyLocationModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "batch_id", Type: model.Text(), Ref: &SupplyBatchModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "quantity", Type: model.Float()},
		{Name: "revision", Type: model.Int()},
		{Name: "updated_at", Type: model.NanoTime(), OmitEmpty: true},
	},
}

// SupplyMovementModel is one append-only entry of the stock ledger. quantity is always > 0; the direction comes from
// movement_type (webtyp.com/stock). There is no update and no delete: a mistake is corrected by a new movement.
var SupplyMovementModel = model.Definition{
	Name: "supply_movement",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, OmitEmpty: true},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "batch_id", Type: model.Text(), Ref: &SupplyBatchModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "location_id", Type: model.Text(), Ref: &SupplyLocationModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "movement_type", Type: model.Text(), NotNull: true},
		{Name: "quantity", Type: model.Float(), NotNull: true},
		{Name: "source_document_type", Type: model.Text(), OmitEmpty: true},
		{Name: "source_document_id", Type: model.Text(), OmitEmpty: true},
		{Name: "note", Type: model.Text(), OmitEmpty: true},
		{Name: "created_at", Type: model.NanoTime()},
		{Name: "created_by", Type: model.Text(), OmitEmpty: true},
	},
}

// Arguments of the operations. Each has tenant_id first, like service_catalog's.

var ListItemsArgsModel = model.Definition{
	Name: "list_items_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "active_only", Type: model.Bool()},
		{Name: "limit", Type: model.Int()},
		{Name: "offset", Type: model.Int()},
	},
}

// IdArgsModel is the argument of every operation that names one record: get, deactivate, delete (item, location).
var IdArgsModel = model.Definition{
	Name: "id_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "id", Type: model.Text()},
	},
}

var ListLocationsArgsModel = model.Definition{
	Name: "list_locations_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "active_only", Type: model.Bool()},
	},
}

var ListBatchesArgsModel = model.Definition{
	Name: "list_batches_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "item_id", Type: model.Text()},
	},
}

var ReceiveStockArgsModel = model.Definition{
	Name: "receive_stock_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "item_id", Type: model.Text()},
		{Name: "location_id", Type: model.Text()},
		{Name: "batch_number", Type: model.Text()},
		{Name: "expiration_date", Type: model.UnixTime()},
		{Name: "unit_cost", Type: model.Int()},
		{Name: "quantity", Type: model.Float()},
		{Name: "source_document_type", Type: model.Text()},
		{Name: "source_document_id", Type: model.Text()},
		{Name: "note", Type: model.Text()},
		{Name: "created_by", Type: model.Text()},
	},
}

var RegisterEntryArgsModel = model.Definition{
	Name: "register_entry_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "item_id", Type: model.Text()},
		{Name: "location_id", Type: model.Text()},
		{Name: "batch_id", Type: model.Text()},
		{Name: "quantity", Type: model.Float()},
		{Name: "note", Type: model.Text()},
		{Name: "created_by", Type: model.Text()},
	},
}

var RegisterOutputArgsModel = model.Definition{
	Name: "register_output_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "item_id", Type: model.Text()},
		{Name: "location_id", Type: model.Text()},
		{Name: "quantity", Type: model.Float()},
		{Name: "movement_type", Type: model.Text()},
		{Name: "batch_id", Type: model.Text()},
		{Name: "source_document_type", Type: model.Text()},
		{Name: "source_document_id", Type: model.Text()},
		{Name: "note", Type: model.Text()},
		{Name: "created_by", Type: model.Text()},
	},
}

var TransferStockArgsModel = model.Definition{
	Name: "transfer_stock_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "item_id", Type: model.Text()},
		{Name: "from_location_id", Type: model.Text()},
		{Name: "to_location_id", Type: model.Text()},
		{Name: "quantity", Type: model.Float()},
		{Name: "note", Type: model.Text()},
		{Name: "created_by", Type: model.Text()},
	},
}

var ListStockArgsModel = model.Definition{
	Name: "list_stock_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "item_id", Type: model.Text()},
		{Name: "location_id", Type: model.Text()},
	},
}

var ListMovementsArgsModel = model.Definition{
	Name: "list_movements_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "item_id", Type: model.Text()},
		{Name: "batch_id", Type: model.Text()},
		{Name: "from", Type: model.NanoTime()},
		{Name: "to", Type: model.NanoTime()},
		{Name: "limit", Type: model.Int()},
	},
}

var ListExpiringArgsModel = model.Definition{
	Name: "list_expiring_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "within_days", Type: model.Int()},
	},
}

var ListLowStockArgsModel = model.Definition{
	Name: "list_low_stock_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
	},
}

// Results that are not a table.

// ExpiringStockModel is one row of list_expiring: stock that expires within the asked days, or already expired.
var ExpiringStockModel = model.Definition{
	Name: "expiring_stock",
	Fields: model.Fields{
		{Name: "item_id", Type: model.Text()},
		{Name: "item_name", Type: model.Text()},
		{Name: "batch_id", Type: model.Text()},
		{Name: "batch_number", Type: model.Text()},
		{Name: "location_id", Type: model.Text()},
		{Name: "quantity", Type: model.Float()},
		{Name: "expiration_date", Type: model.UnixTime()},
		{Name: "expired", Type: model.Bool()},
	},
}

// LowStockModel is one row of list_low_stock: an active item whose total is at or below its min_quantity.
var LowStockModel = model.Definition{
	Name: "low_stock",
	Fields: model.Fields{
		{Name: "item_id", Type: model.Text()},
		{Name: "item_name", Type: model.Text()},
		{Name: "total", Type: model.Float()},
		{Name: "min_quantity", Type: model.Float()},
	},
}
