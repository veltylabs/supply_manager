# Arquitectura

## Alcance

`supply_manager` contiene los insumos clínicos de la clínica (guantes, jeringas, gasas…): qué existe, los lotes en los que vienen (un insumo puede no tener lote), dónde se guardan y cada movimiento de stock. La farmacia es un módulo distinto (`pharmacy_manager`); comparten las reglas en la biblioteca `webtyp.com/stock` pero no comparten tablas ni código, y ninguno importa al otro.

No hay pantalla de stock en esta versión.

## Entidades y Reglas

- **Todo el stock se maneja por lote.** Las reglas de stock (primero en vencer primero en salir, lotes vencidos, nunca negativo, direcciones de movimiento) son las de `webtyp.com/stock`; nada en este módulo las repite.
- **Un movimiento es de solo adición**: no hay actualización ni eliminación; un error se corrige con un nuevo movimiento. Su `quantity` siempre es mayor a cero; la dirección viene del tipo de movimiento.
- **Cada cambio de stock es una transacción** con un control optimista de revisión sobre el nivel de stock (`ErrConflict` cuando la fila cambió).
- **Costo**: un lote tiene un `unit_cost` opcional (pesos enteros por unidad, 0 = desconocido). Con `Deps.Publisher` y `Deps.MoneyScope` configurados, cada movimiento de salida cuyo lote tiene un costo publica un hecho `money.Consumed` (costo × cantidad, redondeado hacia afuera de cero). Una compra nunca es un hecho de este módulo.
- **Eventos**: un `supply.movement.registered` por movimiento, después del commit.
- **Alertas**: se emite una alerta `supply.below_minimum` cuando el stock total de un artículo cruza su mínimo tras una salida. El resumen `supply.expiring` lo emite la revisión diaria `ReviewExpiring` cuando los lotes vencen pronto.
- **Multitenencia (Tenancy)**: toda consulta de una operación de stock filtra por `tenant_id`; las operaciones que maneja la vista genérica resuelven la fila por id y usan el tenant que la fila almacena.
- Unidades: `PIECE`, `BOX`, `PAIR`, `ML`, `GRAM`. Los insumos **no tienen precio, ni tratamiento de impuestos ni marca de venta**.
- El vencimiento de un lote es **opcional** (0 = no vence); un insumo sin lote usa el número de lote `NoBatchNumber` (`NO-BATCH`), pasado explícitamente.
- Salidas aceptadas: `stock.InternalUse` (el uso en una consulta) y `stock.AdjustOut` (un ajuste/baja); `stock.Sale` da `ErrMovementType`. Un lote vencido nunca se usa (`ErrBatchExpired`); solo un ajuste puede tomar stock vencido. `min_quantity` (0 = sin alerta) alimenta `ListLowStock`.

- **Salas (Rooms)**: una ubicación puede nombrar la sala (box) que abastece (`room_id`, una referencia suave a `room_layout`); a lo sumo una ubicación activa por sala y tenant (`ErrRoomTaken`). `UseInRoom` consume de esa ubicación; el selector de sala en el formulario de ubicación vendrá con las pantallas de stock.

## Dependencias

Este módulo depende de:
- `webtyp.com/stock` - Las reglas para el stock se definen aquí (FEFO, vencimiento, no stock negativo).
- `webtyp.com/money` - Se usa para registrar hechos `money.Consumed` en los movimientos de salida.

## Ejemplo de Raíz de Composición (Composition Root)

```go
m, err := supplymanager.New(db, supplymanager.Deps{
	IDs:        ids,
	Publisher:  broker,
	MoneyScope: "supply",
})
```

## Operaciones de Vista Genérica

Por qué `tenant_id` está ausente de las operaciones que maneja la vista genérica: resuelven la fila por id y usan el tenant que la fila almacena.
