# supply_manager
<img src="docs/img/badges.svg">

Este módulo maneja los insumos clínicos de la clínica (como guantes, jeringas y gasas). Gestiona los artículos, sus lotes (un insumo puede no tener lote, en cuyo caso usa `NoBatchNumber`; la fecha de vencimiento es opcional), las ubicaciones donde se guardan, y el libro mayor de movimientos de solo adición.

Los insumos no se venden y no tienen precio, y lo que se usa en una consulta es registrado por `clinical_encounter` como una salida `stock.InternalUse` que lleva el id de la consulta.

## Inicio Rápido

```go
package main

import (
	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/events"
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/stock"
)

func example(db *orm.DB, ids model.IDGenerator, broker events.Publisher) {
	// Build the module
	m, err := supplymanager.New(db, supplymanager.Deps{IDs: ids, Publisher: broker, MoneyScope: "supply"})

	// Create a location
	loc, _ := m.CreateLocation(supplymanager.SupplyLocation{TenantId: "t1", Name: "Store", IsActive: true})

	// Create an item
	item, _ := m.CreateItem(supplymanager.SupplyItem{TenantId: "t1", Sku: "123", Name: "Gloves", Unit: supplymanager.UnitBox, IsActive: true})

	// Receive stock (Recibir stock)
	m.ReceiveStock(supplymanager.ReceiveStockArgs{TenantId: "t1", ItemId: item.Id, LocationId: loc.Id, BatchNumber: supplymanager.NoBatchNumber, Quantity: 10}) // sin lote, sin vencimiento

	// Register an output (Registrar una salida)
	m.RegisterOutput(supplymanager.RegisterOutputArgs{TenantId: "t1", ItemId: item.Id, LocationId: loc.Id, Quantity: 2, MovementType: string(stock.InternalUse)})
}
```

## Operaciones

Cada operación se llama `supply.<name>` y declara su recurso y acción RBAC en `MountOperations`.

| Operación | Constante | Recurso | Acción |
|---|---|---|---|
| `list_items` | `OpListItems` | `supply_item` | read |
| `get_item` | `OpGetItem` | `supply_item` | read |
| `create_item` | `OpCreateItem` | `supply_item` | create |
| `update_item` | `OpUpdateItem` | `supply_item` | update |
| `upsert_item` | `OpUpsertItem` | `supply_item` | create + update |
| `deactivate_item` | `OpDeactivateItem` | `supply_item` | update |
| `delete_item` | `OpDeleteItem` | `supply_item` | delete |
| `list_locations` | `OpListLocations` | `supply_location` | read |
| `upsert_location` | `OpUpsertLocation` | `supply_location` | create + update |
| `update_location` | `OpUpdateLocation` | `supply_location` | update |
| `delete_location` | `OpDeleteLocation` | `supply_location` | delete |
| `list_batches` | `OpListBatches` | `supply_batch` | read |
| `receive_stock` | `OpReceiveStock` | `supply_movement` | create |
| `register_entry` | `OpRegisterEntry` | `supply_movement` | create |
| `register_output` | `OpRegisterOutput` | `supply_movement` | create |
| `transfer_stock` | `OpTransferStock` | `supply_movement` | create |
| `list_stock` | `OpListStock` | `supply_stock_level` | read |
| `list_movements` | `OpListMovements` | `supply_movement` | read |
| `list_expiring` | `OpListExpiring` | `supply_stock_level` | read |
| `list_low_stock` | `OpListLowStock` | `supply_stock_level` | read |

## Lectores (Readers)

`ListExpiring` y `ListLowStock` son reportes. Para otros módulos: `ItemExists(tenantId, itemId) (bool, error)` (existe en el tenant y está activo) y `ItemCard`, lo que otro módulo captura de un artículo; un artículo ausente o foráneo devuelve `ErrNotFound`. Un módulo que los necesita declara el método en una interfaz reducida propia.

`UseInRoom(tenantId, roomId, itemId string, quantity float64, docType, docId, by string) (cost int64, err error)` toma un consumo (`stock.InternalUse`, primero en vencer primero en salir) de la ubicación activa que abastece a la sala (box) `roomId`, registra el documento que lo causó y devuelve su costo en pesos enteros. `clinical_encounter` lo usa cuando una atención se marca como terminada; la sala viene de `room_layout`. Si no hay ubicación para la sala devuelve `ErrNoLocationForRoom`.

## Hechos de Costo (Cost facts)

Si se establece `Deps.MoneyScope` y un lote tiene un costo unitario, cada movimiento de salida publica un hecho `money.Consumed` (costo × cantidad, redondeado hacia afuera del cero). Una compra nunca es un hecho de este módulo.

## Alertas

Este módulo puede emitir alertas cuando `Deps.Publisher` y `Deps.AlertArea` están configurados:
- **Por debajo del mínimo:** Tras una salida, si el stock total del artículo pasa de estar por encima de su `min_quantity` a estar en o por debajo, se emite una alerta `supply.below_minimum`. Mantenerse bajo no repite la alerta. Las transferencias no disparan esto.
- **Por vencer:** La aplicación puede llamar a `ReviewExpiring(tenantId, withinDays)` diariamente para revisar el stock. Emitirá una alerta de resumen `supply.expiring` si al menos un lote con stock está vencido o vence dentro de esa cantidad de días.
- La aplicación debe registrar `AlertKinds(area)` para añadir las definiciones de alerta a su `notify.Registry`.

## Archivos clave

| Archivo | Qué es |
|---|---|
| `model.go`, `model_orm.go` | las cinco tablas y cada modelo de argumento; `model_orm.go` es generado por `ormc`, nunca editarlo a mano |
| `errors.go` | cada constante de error (todos los mensajes empiezan con `supply: `), `statusOf`, `writeError` |
| `module.go` | `Deps`, `Module`, `New`, y las cuatro operaciones de stock `ReceiveStock`, `RegisterEntry`, `RegisterOutput`, `TransferStock`; hechos de costo y eventos |
| `crud.go` | actualización / upsert / patch / desactivación / borrado de artículo y ubicación |
| `queries.go`, `reports.go` | las listas, `ListExpiring`, `ListLowStock` y los lectores |
| `ops.go` | los nombres de operación, las formas de red que envía la vista genérica, `MountOperations` y cada manejador |
| `view.go` | `NewItemView`, `NewLocationView` |
| `migrate/migrate.go` | crea las cinco tablas en orden de clave foránea |
| `ui/`, `seed/`, `web/` | el grupo de pantallas "Insumos", los datos de demostración, la demo en el navegador |
| `lang.json` | el español de las etiquetas del formulario y de las opciones (generado con `langc sync .`) |
| `conflict_internal_test.go` | el test de revisión obsoleta (a nivel raíz, justificado) |
| `tests/setup_test.go` | ayudantes: `newModule`, `newItem`, `newLocation`, `testTenant`, `testIDGen` |
| `tests/receive_test.go`, `output_test.go`, `reports_test.go`, `ops_test.go`, `ui_test.go`, `seed_test.go`, `mount_test.go`, `conformance_test.go`, `ledger_test.go` | tests que existen: úsalos como el estilo de los tuyos |
