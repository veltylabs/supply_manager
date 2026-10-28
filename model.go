package supplymanager

import (
	"webtyp.com/input"
	"webtyp.com/model"
)

// ModelName es el prefijo en red y RBAC del módulo: las operaciones se llaman "<ModelName>.<op>".
const ModelName = "supply"

// Units en las que se cuenta un artículo. El select del formulario toma sus opciones de estas (ui/browser.go, SetOptions).
const (
	UnitPiece = "PIECE"
	UnitBox   = "BOX"
	UnitPair  = "PAIR"
	UnitMl    = "ML"
	UnitGram  = "GRAM"
)

// NoBatchNumber es el número de lote de un insumo que no tiene lote: se pasa explícitamente a ReceiveStock.
const NoBatchNumber = "NO-BATCH"

// SupplyItemModel es un insumo clínico (guantes, jeringas, gasas). No se vende y no tiene precio aquí.
var SupplyItemModel = model.Definition{
	Name: "supply_item",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, OmitEmpty: true},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "sku", Type: input.Text(), Label: "SKU", Item: model.ItemDescription, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Extra: []rune{'-'}, Minimum: 1, Maximum: 50}},
		{Name: "name", Type: input.Text(), Label: "Name", Item: model.ItemLabel, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Spaces: true, Tilde: true, Extra: []rune{'.', ',', '(', ')', '-', '/', '%', '+'}, Minimum: 1, Maximum: 255}},
		{Name: "unit", Type: input.Select(), Label: "Unit", NotNull: true},
		{Name: "min_quantity", Type: input.Decimal(), Label: "Minimum quantity", OmitEmpty: true},
		{Name: "is_active", Type: input.Checkbox(), Label: "Active"},
		{Name: "updated_at", Type: model.NanoTime(), OmitEmpty: true},
	},
}

// SupplyLocationModel es un lugar donde se guarda el stock: un almacén, el mostrador.
var SupplyLocationModel = model.Definition{
	Name: "supply_location",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, OmitEmpty: true},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "name", Type: input.Text(), Label: "Name", Item: model.ItemLabel, NotNull: true, Permitted: model.Permitted{Letters: true, Numbers: true, Spaces: true, Tilde: true, Minimum: 1, Maximum: 100}},
		// room_id es la sala (box) que esta ubicación abastece, una referencia suave a room_layout; "" = ninguna. A lo sumo una
		// ubicación activa por sala. Aún no hay widget: el selector de sala vendrá con las pantallas de stock.
		{Name: "room_id", Type: model.Text(), OmitEmpty: true},
		{Name: "is_active", Type: input.Checkbox(), Label: "Active"},
		{Name: "updated_at", Type: model.NanoTime(), OmitEmpty: true},
	},
}

// SupplyBatchModel es un lote de un artículo (un insumo sin lote usa NoBatchNumber). expiration_date es segundos Unix UTC, 0 = no vence; unit_cost es pesos enteros por unidad, 0 = desconocido.
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

// SupplyStockLevelModel es cuánto de un lote hay en una ubicación. Una fila por (ubicación, lote); la revisión protege
// contra actualizaciones concurrentes.
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

// SupplyMovementModel es una entrada de solo adición en el libro mayor de stock. quantity siempre es > 0; la dirección viene de
// movement_type (webtyp.com/stock). No hay actualización ni borrado: un error se corrige con un nuevo movimiento.
var SupplyMovementModel = model.Definition{
	Name: "supply_movement",
	Fields: model.Fields{
		{Name: "id", Type: model.Text(), DB: &model.FieldDB{PK: true}, OmitEmpty: true},
		{Name: "tenant_id", Type: model.Text(), NotNull: true},
		{Name: "batch_id", Type: model.Text(), Ref: &SupplyBatchModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "location_id", Type: model.Text(), Ref: &SupplyLocationModel, DB: &model.FieldDB{RefColumn: "id"}, NotNull: true},
		{Name: "movement_type", Type: model.Text(), NotNull: true},
		{Name: "quantity", Type: model.Float(), NotNull: true},
		{Name: "source_document_type", Type: model.Text(), Label: "Document type", OmitEmpty: true},
		{Name: "source_document_id", Type: model.Text(), Label: "Document ID", OmitEmpty: true},
		{Name: "note", Type: model.Text(), OmitEmpty: true},
		{Name: "created_at", Type: model.NanoTime()},
		{Name: "created_by", Type: model.Text(), OmitEmpty: true},
	},
}

// Argumentos de las operaciones. Cada uno tiene tenant_id primero, como los de service_catalog.

var ListItemsArgsModel = model.Definition{
	Name: "list_items_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "active_only", Type: model.Bool()},
		{Name: "limit", Type: model.Int()},
		{Name: "offset", Type: model.Int()},
	},
}

// IdArgsModel es el argumento de cada operación que nombra un registro: obtener, desactivar, borrar (artículo, ubicación).
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
		{Name: "source_document_type", Type: model.Text(), Label: "Document type"},
		{Name: "source_document_id", Type: model.Text(), Label: "Document ID"},
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
		{Name: "source_document_type", Type: model.Text(), Label: "Document type"},
		{Name: "source_document_id", Type: model.Text(), Label: "Document ID"},
		{Name: "note", Type: model.Text()},
		{Name: "created_by", Type: model.Text()},
	},
}

var TransferStockArgsModel = model.Definition{
	Name: "transfer_stock_args",
	Fields: model.Fields{
		{Name: "tenant_id", Type: model.Text()},
		{Name: "item_id", Type: model.Text()},
		{Name: "from_location_id", Type: model.Text(), Label: "Origin location"},
		{Name: "to_location_id", Type: model.Text(), Label: "Target location"},
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

// Resultados que no son una tabla.

// ExpiringStockModel es una fila de list_expiring: stock que vence dentro de los días solicitados, o ya vencido.
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

// LowStockModel es una fila de list_low_stock: un artículo activo cuyo total está en o por debajo de su min_quantity.
var LowStockModel = model.Definition{
	Name: "low_stock",
	Fields: model.Fields{
		{Name: "item_id", Type: model.Text()},
		{Name: "item_name", Type: model.Text()},
		{Name: "total", Type: model.Float()},
		{Name: "min_quantity", Type: model.Float()},
	},
}
