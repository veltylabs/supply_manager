package supplymanager

import (
	"webtyp.com/events"
	"webtyp.com/fmt"
	"webtyp.com/model"
	"webtyp.com/money"
	"webtyp.com/notify"
	"webtyp.com/orm"
	"webtyp.com/stock"
	"webtyp.com/time"
)

// TopicMovementRegistered se publica una vez por cada movimiento de stock, después del commit. Payload: *SupplyMovement.
const TopicMovementRegistered = "supply.movement.registered"

const (
	// sourceType es el Ref.Type de los hechos de costo: el movimiento es su causa.
	sourceType = "supply_movement"

	msgNeedIDs     = "supply: Deps.IDs is required"
	msgPublishCost = "supply: the stock change was saved but its cost fact could not be published:"
)

// Deps son las dependencias que el módulo necesita de la aplicación que lo compone.
type Deps struct {
	IDs        model.IDGenerator // requerido: el módulo nunca construye un generador
	Publisher  events.Publisher  // opcional: nil desactiva los eventos y los hechos de costo
	MoneyScope string            // opcional: el área del negocio a la que pertenecen los hechos de costo; "" desactiva los hechos de costo
	AlertArea  string            // opcional: el código notify.Area de la aplicación para las alertas de este módulo; "" las desactiva
}

type Module struct {
	db        *orm.DB
	ids       model.IDGenerator
	pub       events.Publisher
	scope     string
	alertArea string
}

func New(db *orm.DB, deps Deps) (*Module, error) {
	if deps.IDs == nil {
		return nil, fmt.Err(msgNeedIDs)
	}
	return &Module{db: db, ids: deps.IDs, pub: deps.Publisher, scope: deps.MoneyScope, alertArea: deps.AlertArea}, nil
}

// ModelName devuelve el nombre del modelo.
func (m *Module) ModelName() string { return ModelName }

// ---- items and locations (el patrón que sigue el resto del CRUD) ----

// CreateItem crea un nuevo artículo.
//
// Ejemplo:
//  item, err := m.CreateItem(SupplyItem{TenantId: "t1", Sku: "123", Name: "Guantes", Unit: UnitBox, IsActive: true})
func (m *Module) CreateItem(p SupplyItem) (SupplyItem, error) {
	if p.Id == "" {
		p.Id = m.ids.NewID()
	}
	p.UpdatedAt = time.NowNano()
	if err := checkItem(&p); err != nil {
		return SupplyItem{}, err
	}
	if err := p.Validate(model.ActionCreate); err != nil {
		return SupplyItem{}, err
	}
	if _, found, err := m.itemBySKU(m.db, p.TenantId, p.Sku); err != nil {
		return SupplyItem{}, err
	} else if found {
		return SupplyItem{}, ErrSkuExists
	}
	if err := m.db.Create(&p); err != nil {
		return SupplyItem{}, err
	}
	return p, nil
}

// checkItem aplica las reglas de un artículo que las restricciones de los campos no pueden expresar.
func checkItem(p *SupplyItem) error {
	switch p.Unit {
	case UnitPiece, UnitBox, UnitPair, UnitMl, UnitGram:
	default:
		return ErrUnit
	}
	return nil
}

// GetItem devuelve un artículo por su id.
//
// Ejemplo:
//  item, err := m.GetItem("t1", "1")
func (m *Module) GetItem(tenantId, id string) (SupplyItem, error) {
	return getItem(m.db, tenantId, id)
}

func getItem(db *orm.DB, tenantId, id string) (SupplyItem, error) {
	var p SupplyItem
	qb := db.Query(&p).Where(SupplyItem_.Id).Eq(id).Where(SupplyItem_.TenantId).Eq(tenantId)
	got, err := ReadOneSupplyItem(qb, &p)
	if orm.IsNotFound(err) {
		return SupplyItem{}, ErrItemNotFound
	}
	if err != nil {
		return SupplyItem{}, err
	}
	return *got, nil
}

func (m *Module) itemBySKU(db *orm.DB, tenantId, sku string) (SupplyItem, bool, error) {
	var p SupplyItem
	qb := db.Query(&p).Where(SupplyItem_.Sku).Eq(sku).Where(SupplyItem_.TenantId).Eq(tenantId)
	got, err := ReadOneSupplyItem(qb, &p)
	if orm.IsNotFound(err) {
		return SupplyItem{}, false, nil
	}
	if err != nil {
		return SupplyItem{}, false, err
	}
	return *got, true, nil
}

// CreateLocation crea una nueva ubicación.
//
// Ejemplo:
//  loc, err := m.CreateLocation(SupplyLocation{TenantId: "t1", Name: "Depósito", IsActive: true})
func (m *Module) CreateLocation(l SupplyLocation) (SupplyLocation, error) {
	if l.Id == "" {
		l.Id = m.ids.NewID()
	}
	l.UpdatedAt = time.NowNano()
	if err := l.Validate(model.ActionCreate); err != nil {
		return SupplyLocation{}, err
	}
	var proxy SupplyLocation
	qb := m.db.Query(&proxy).Where(SupplyLocation_.Name).Eq(l.Name).Where(SupplyLocation_.TenantId).Eq(l.TenantId)
	if _, err := ReadOneSupplyLocation(qb, &proxy); err == nil {
		return SupplyLocation{}, ErrLocationNameExists
	} else if !orm.IsNotFound(err) {
		return SupplyLocation{}, err
	}
	if err := m.checkRoomFree(l); err != nil {
		return SupplyLocation{}, err
	}
	if err := m.db.Create(&l); err != nil {
		return SupplyLocation{}, err
	}
	return l, nil
}

// GetLocation devuelve una ubicación por su id.
//
// Ejemplo:
//  loc, err := m.GetLocation("t1", "1")
func (m *Module) GetLocation(tenantId, id string) (SupplyLocation, error) {
	return getLocation(m.db, tenantId, id)
}

func getLocation(db *orm.DB, tenantId, id string) (SupplyLocation, error) {
	var l SupplyLocation
	qb := db.Query(&l).Where(SupplyLocation_.Id).Eq(id).Where(SupplyLocation_.TenantId).Eq(tenantId)
	got, err := ReadOneSupplyLocation(qb, &l)
	if orm.IsNotFound(err) {
		return SupplyLocation{}, ErrLocationNotFound
	}
	if err != nil {
		return SupplyLocation{}, err
	}
	return *got, nil
}

// ---- lotes y niveles de stock ----

func getBatch(db *orm.DB, tenantId, id string) (SupplyBatch, error) {
	var b SupplyBatch
	qb := db.Query(&b).Where(SupplyBatch_.Id).Eq(id).Where(SupplyBatch_.TenantId).Eq(tenantId)
	got, err := ReadOneSupplyBatch(qb, &b)
	if orm.IsNotFound(err) {
		return SupplyBatch{}, ErrBatchNotFound
	}
	if err != nil {
		return SupplyBatch{}, err
	}
	return *got, nil
}

func batchByNumber(db *orm.DB, tenantId, itemId, number string) (SupplyBatch, bool, error) {
	var b SupplyBatch
	qb := db.Query(&b).Where(SupplyBatch_.ItemId).Eq(itemId).Where(SupplyBatch_.BatchNumber).Eq(number).Where(SupplyBatch_.TenantId).Eq(tenantId)
	got, err := ReadOneSupplyBatch(qb, &b)
	if orm.IsNotFound(err) {
		return SupplyBatch{}, false, nil
	}
	if err != nil {
		return SupplyBatch{}, false, err
	}
	return *got, true, nil
}

// levelAt es el nivel de stock de un lote en una ubicación; found es false cuando el lote nunca estuvo allí.
func levelAt(db *orm.DB, tenantId, locationId, batchId string) (SupplyStockLevel, bool, error) {
	var l SupplyStockLevel
	qb := db.Query(&l).Where(SupplyStockLevel_.LocationId).Eq(locationId).Where(SupplyStockLevel_.BatchId).Eq(batchId).Where(SupplyStockLevel_.TenantId).Eq(tenantId)
	got, err := ReadOneSupplyStockLevel(qb, &l)
	if orm.IsNotFound(err) {
		return SupplyStockLevel{}, false, nil
	}
	if err != nil {
		return SupplyStockLevel{}, false, err
	}
	return *got, true, nil
}

// levelOrCreate devuelve el nivel de stock de un lote en una ubicación, creándolo con cantidad 0 y revisión 0.
func (m *Module) levelOrCreate(tx *orm.DB, tenantId, locationId, batchId string, now time.Nano) (SupplyStockLevel, error) {
	level, found, err := levelAt(tx, tenantId, locationId, batchId)
	if err != nil {
		return SupplyStockLevel{}, err
	}
	if found {
		return level, nil
	}
	level = SupplyStockLevel{Id: m.ids.NewID(), TenantId: tenantId, LocationId: locationId, BatchId: batchId, UpdatedAt: now}
	if err := tx.Create(&level); err != nil {
		return SupplyStockLevel{}, err
	}
	return level, nil
}

// applyToLevel cambia la cantidad de un nivel de stock por `quantity` en la dirección d, protegido por la revisión que el llamador
// leyó: si el registro cambió desde entonces (expectedRevision difiere de la almacenada) devuelve ErrConflict y no se escribe nada.
// stock.Apply evita que la cantidad sea negativa (ErrInsufficientStock).
func applyToLevel(tx *orm.DB, level *SupplyStockLevel, expectedRevision int64, d stock.Direction, quantity float64, now time.Nano) error {
	var current SupplyStockLevel
	qb := tx.Query(&current).Where(SupplyStockLevel_.Id).Eq(level.Id).Where(SupplyStockLevel_.TenantId).Eq(level.TenantId)
	got, err := ReadOneSupplyStockLevel(qb, &current)
	if err != nil {
		return err
	}
	if got.Revision != expectedRevision {
		return ErrConflict
	}
	newQuantity, err := stock.Apply(got.Quantity, d, quantity)
	if err == stock.ErrInsufficient {
		return ErrInsufficientStock
	}
	if err != nil {
		return err
	}
	got.Quantity = newQuantity
	got.Revision++
	got.UpdatedAt = now
	if err := tx.Update(got, orm.Eq(SupplyStockLevel_.Id, got.Id), orm.Eq(SupplyStockLevel_.TenantId, got.TenantId)); err != nil {
		return err
	}
	*level = *got
	return nil
}

// addMovement agrega una entrada al libro mayor dentro de la transacción.
func (m *Module) addMovement(tx *orm.DB, tenantId, batchId, locationId string, t stock.Movement, quantity float64, docType, docId, note, by string, now time.Nano) (SupplyMovement, error) {
	mv := SupplyMovement{
		Id: m.ids.NewID(), TenantId: tenantId, BatchId: batchId, LocationId: locationId, MovementType: string(t), Quantity: quantity,
		SourceDocumentType: docType, SourceDocumentId: docId, Note: note, CreatedAt: now, CreatedBy: by,
	}
	if err := tx.Create(&mv); err != nil {
		return SupplyMovement{}, err
	}
	return mv, nil
}

// lotsAt convierte el stock de un artículo en una ubicación en los lotes de los que webtyp.com/stock asigna: un Lot por lote
// que tiene un nivel allí, con el vencimiento del lote y la cantidad del nivel.
func lotsAt(db *orm.DB, tenantId, itemId, locationId string) ([]stock.Lot, error) {
	var proxy SupplyBatch
	batches, err := ReadAllSupplyBatch(db.Query(&proxy).Where(SupplyBatch_.ItemId).Eq(itemId).Where(SupplyBatch_.TenantId).Eq(tenantId))
	if err != nil {
		return nil, err
	}
	lots := make([]stock.Lot, 0, len(batches))
	for _, b := range batches {
		level, found, err := levelAt(db, tenantId, locationId, b.Id)
		if err != nil {
			return nil, err
		}
		if !found {
			continue
		}
		lots = append(lots, stock.Lot{ID: b.Id, Expiry: b.ExpirationDate, Quantity: level.Quantity})
	}
	return lots, nil
}

// ---- the four stock operations ----

// ReceiveStock añade stock de un artículo a una ubicación: encuentra o crea el lote (la fecha de vencimiento es opcional, 0 = ninguna), añade
// la cantidad al nivel (ubicación, lote) y añade un movimiento stock.Purchase, todo en una sola transacción.
//
// Ejemplo:
//  mov, err := m.ReceiveStock(ReceiveStockArgs{TenantId: "t1", ItemId: "1", LocationId: "2", Quantity: 10, BatchNumber: NoBatchNumber})
func (m *Module) ReceiveStock(a ReceiveStockArgs) (SupplyMovement, error) {
	if a.Quantity <= 0 {
		return SupplyMovement{}, model.Invalid(stock.ErrQuantity)
	}
	if a.BatchNumber == "" {
		return SupplyMovement{}, ErrBatchNumber
	}
	if a.ExpirationDate < 0 {
		return SupplyMovement{}, ErrExpiry
	}
	if a.UnitCost < 0 {
		return SupplyMovement{}, ErrUnitCost
	}
	now := time.NowNano()
	var mv SupplyMovement
	err := m.db.Tx(func(tx *orm.DB) error {
		item, err := getItem(tx, a.TenantId, a.ItemId)
		if err != nil {
			return err
		}
		if !item.IsActive {
			return ErrItemInactive
		}
		location, err := getLocation(tx, a.TenantId, a.LocationId)
		if err != nil {
			return err
		}
		if !location.IsActive {
			return ErrLocationInactive
		}
		batch, found, err := batchByNumber(tx, a.TenantId, a.ItemId, a.BatchNumber)
		if err != nil {
			return err
		}
		if found {
			if batch.ExpirationDate != a.ExpirationDate {
				return ErrBatchExpiryMismatch
			}
			if a.UnitCost != 0 && batch.UnitCost != 0 && batch.UnitCost != a.UnitCost {
				return ErrBatchCostMismatch
			}
		} else {
			batch = SupplyBatch{
				Id: m.ids.NewID(), TenantId: a.TenantId, ItemId: a.ItemId, BatchNumber: a.BatchNumber,
				ExpirationDate: a.ExpirationDate, UnitCost: a.UnitCost, IsActive: true, UpdatedAt: now,
			}
			if err := tx.Create(&batch); err != nil {
				return err
			}
		}
		level, err := m.levelOrCreate(tx, a.TenantId, a.LocationId, batch.Id, now)
		if err != nil {
			return err
		}
		if err := applyToLevel(tx, &level, level.Revision, stock.In, a.Quantity, now); err != nil {
			return err
		}
		mv, err = m.addMovement(tx, a.TenantId, batch.Id, a.LocationId, stock.Purchase, a.Quantity, a.SourceDocumentType, a.SourceDocumentId, a.Note, a.CreatedBy, now)
		return err
	})
	if err != nil {
		return SupplyMovement{}, err
	}
	m.publish(mv)
	return mv, nil
}

// RegisterEntry añade a un lote EXISTENTE: una corrección de entrada (stock.AdjustIn).
//
// Ejemplo:
//  mov, err := m.RegisterEntry(RegisterEntryArgs{TenantId: "t1", ItemId: "1", LocationId: "2", BatchId: "3", Quantity: 5})
func (m *Module) RegisterEntry(a RegisterEntryArgs) (SupplyMovement, error) {
	if a.Quantity <= 0 {
		return SupplyMovement{}, model.Invalid(stock.ErrQuantity)
	}
	now := time.NowNano()
	var mv SupplyMovement
	err := m.db.Tx(func(tx *orm.DB) error {
		if _, err := getItem(tx, a.TenantId, a.ItemId); err != nil {
			return err
		}
		if _, err := getLocation(tx, a.TenantId, a.LocationId); err != nil {
			return err
		}
		batch, err := getBatch(tx, a.TenantId, a.BatchId)
		if err != nil {
			return err
		}
		if batch.ItemId != a.ItemId {
			return ErrBatchNotFound
		}
		level, err := m.levelOrCreate(tx, a.TenantId, a.LocationId, batch.Id, now)
		if err != nil {
			return err
		}
		if err := applyToLevel(tx, &level, level.Revision, stock.In, a.Quantity, now); err != nil {
			return err
		}
		mv, err = m.addMovement(tx, a.TenantId, batch.Id, a.LocationId, stock.AdjustIn, a.Quantity, "", "", a.Note, a.CreatedBy, now)
		return err
	})
	if err != nil {
		return SupplyMovement{}, err
	}
	m.publish(mv)
	return mv, nil
}

// RegisterOutput saca stock: un uso interno (stock.InternalUse, el uso en una consulta) o una baja (stock.AdjustOut). Sin un id de lote, los lotes se extraen por
// primero en vencer, primero en salir (stock.Allocate); solo una baja (stock.AdjustOut) puede tomar stock vencido. Devuelve un
// movimiento por cada lote usado. Si el hecho de costo no se puede publicar después del commit, los movimientos se devuelven CON el
// error: el cambio de stock ya está guardado.
//
// Ejemplo:
//  movs, err := m.RegisterOutput(RegisterOutputArgs{TenantId: "t1", ItemId: "1", LocationId: "2", Quantity: 2, MovementType: string(stock.InternalUse)})
const msgBelowMinimum = "%v %s left; minimum %v."

func (m *Module) RegisterOutput(a RegisterOutputArgs) ([]SupplyMovement, error) {
	if a.Quantity <= 0 {
		return nil, model.Invalid(stock.ErrQuantity)
	}
	t := stock.Movement(a.MovementType)
	if t != stock.InternalUse && t != stock.AdjustOut {
		return nil, ErrMovementType
	}

	before, err := m.totalItem(a.TenantId, a.ItemId)
	if err != nil {
		return nil, err
	}

	now := time.NowNano()
	var out []SupplyMovement
	var costs []int64 // unit cost of the batch of each movement, same order as out
	err = m.db.Tx(func(tx *orm.DB) error {
		if _, err := getItem(tx, a.TenantId, a.ItemId); err != nil {
			return err
		}
		if _, err := getLocation(tx, a.TenantId, a.LocationId); err != nil {
			return err
		}

		var takes []stock.Take
		if a.BatchId == "" {
			lots, err := lotsAt(tx, a.TenantId, a.ItemId, a.LocationId)
			if err != nil {
				return err
			}
			allocateAt := time.NowUnix()
			if t == stock.AdjustOut {
				allocateAt = 0 // a write-off may take expired stock: with now = 0 no lot counts as expired
			}
			takes, err = stock.Allocate(lots, a.Quantity, allocateAt)
			if err == stock.ErrInsufficient {
				return ErrInsufficientStock
			}
			if err != nil {
				return err
			}
		} else {
			batch, err := getBatch(tx, a.TenantId, a.BatchId)
			if err != nil {
				return err
			}
			if batch.ItemId != a.ItemId {
				return ErrBatchNotFound
			}
			level, found, err := levelAt(tx, a.TenantId, a.LocationId, batch.Id)
			if err != nil {
				return err
			}
			if !found || level.Quantity < a.Quantity {
				return ErrInsufficientStock
			}
			if t != stock.AdjustOut && (stock.Lot{Expiry: batch.ExpirationDate}).Expired(time.NowUnix()) {
				return ErrBatchExpired
			}
			takes = []stock.Take{{LotID: batch.Id, Quantity: a.Quantity}}
		}

		for _, take := range takes {
			batch, err := getBatch(tx, a.TenantId, take.LotID)
			if err != nil {
				return err
			}
			level, found, err := levelAt(tx, a.TenantId, a.LocationId, take.LotID)
			if err != nil {
				return err
			}
			if !found {
				return ErrInsufficientStock
			}
			if err := applyToLevel(tx, &level, level.Revision, stock.Out, take.Quantity, now); err != nil {
				return err
			}
			mv, err := m.addMovement(tx, a.TenantId, take.LotID, a.LocationId, t, take.Quantity, a.SourceDocumentType, a.SourceDocumentId, a.Note, a.CreatedBy, now)
			if err != nil {
				return err
			}
			out = append(out, mv)
			costs = append(costs, batch.UnitCost)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, mv := range out {
		m.publish(mv)
	}
	if err := m.publishCosts(out, costs); err != nil {
		return out, err
	}

	if len(out) > 0 {
		item, err := m.GetItem(a.TenantId, a.ItemId)
		if err == nil {
			after, err := m.totalItem(a.TenantId, a.ItemId)
			if err == nil && stock.CrossedLow(before, after, item.MinQuantity) {
				alertErr := m.raise(notify.Alert{
					Code:    AlertBelowMinimum,
					Subject: item.Name,
					Body:    fmt.Sprintf(msgBelowMinimum, after, item.Unit, item.MinQuantity),
					Source:  notify.Ref{Type: alertSourceItem, ID: item.Id},
				})
				if alertErr != nil {
					return out, alertErr
				}
			} else if err != nil {
				return out, err
			}
		} else {
			return out, err
		}
	}

	return out, nil
}

// TransferStock mueve el stock de un artículo entre dos ubicaciones, sacando los lotes por primero en vencer, primero en salir y
// poniendo los mismos lotes en el destino: movimientos stock.TransferOut en el origen, stock.TransferIn en el
// destino, en una sola transacción.
//
// Ejemplo:
//  movs, err := m.TransferStock(TransferStockArgs{TenantId: "t1", ItemId: "1", FromLocationId: "2", ToLocationId: "3", Quantity: 5})
func (m *Module) TransferStock(a TransferStockArgs) ([]SupplyMovement, error) {
	if a.Quantity <= 0 {
		return nil, model.Invalid(stock.ErrQuantity)
	}
	if a.FromLocationId == a.ToLocationId {
		return nil, ErrSameLocation
	}
	now := time.NowNano()
	var out []SupplyMovement
	err := m.db.Tx(func(tx *orm.DB) error {
		if _, err := getItem(tx, a.TenantId, a.ItemId); err != nil {
			return err
		}
		if _, err := getLocation(tx, a.TenantId, a.FromLocationId); err != nil {
			return err
		}
		to, err := getLocation(tx, a.TenantId, a.ToLocationId)
		if err != nil {
			return err
		}
		if !to.IsActive {
			return ErrLocationInactive
		}
		lots, err := lotsAt(tx, a.TenantId, a.ItemId, a.FromLocationId)
		if err != nil {
			return err
		}
		takes, err := stock.Allocate(lots, a.Quantity, time.NowUnix())
		if err == stock.ErrInsufficient {
			return ErrInsufficientStock
		}
		if err != nil {
			return err
		}
		for _, take := range takes {
			src, found, err := levelAt(tx, a.TenantId, a.FromLocationId, take.LotID)
			if err != nil {
				return err
			}
			if !found {
				return ErrInsufficientStock
			}
			if err := applyToLevel(tx, &src, src.Revision, stock.Out, take.Quantity, now); err != nil {
				return err
			}
			dst, err := m.levelOrCreate(tx, a.TenantId, a.ToLocationId, take.LotID, now)
			if err != nil {
				return err
			}
			if err := applyToLevel(tx, &dst, dst.Revision, stock.In, take.Quantity, now); err != nil {
				return err
			}
			mvOut, err := m.addMovement(tx, a.TenantId, take.LotID, a.FromLocationId, stock.TransferOut, take.Quantity, "", "", a.Note, a.CreatedBy, now)
			if err != nil {
				return err
			}
			mvIn, err := m.addMovement(tx, a.TenantId, take.LotID, a.ToLocationId, stock.TransferIn, take.Quantity, "", "", a.Note, a.CreatedBy, now)
			if err != nil {
				return err
			}
			out = append(out, mvOut, mvIn)
		}
		return nil
	})
	if err != nil {
		return nil, err
	}
	for _, mv := range out {
		m.publish(mv)
	}
	return out, nil
}

// ---- events and cost facts ----

func (m *Module) publish(mv SupplyMovement) {
	if m.pub != nil {
		m.pub.Publish(events.Event{Topic: TopicMovementRegistered, Payload: &mv})
	}
}

// publishCosts anuncia lo que un consumo gastó, al costo: un hecho money.Consumed por cada movimiento cuyo lote tenga un costo unitario.
// No hace nada a menos que la aplicación proporcione un Publisher y un MoneyScope.
func (m *Module) publishCosts(movements []SupplyMovement, unitCosts []int64) error {
	if m.pub == nil || m.scope == "" {
		return nil
	}
	for i, mv := range movements {
		amount := int64(float64(unitCosts[i])*mv.Quantity + 0.5) // half away from zero; both factors are positive
		if amount <= 0 {
			continue
		}
		price, err := money.New(amount, money.CLP)
		if err != nil {
			return fmt.Err(msgPublishCost, err)
		}
		fact := money.Fact{
			Kind:   money.Consumed,
			Side:   money.Outflow,
			Amount: price,
			Scope:  m.scope,
			Source: money.Ref{Type: sourceType, ID: mv.Id},
			At:     mv.CreatedAt.Unix(),
		}
		if err := money.Publish(m.pub, fact); err != nil {
			return fmt.Err(msgPublishCost, err)
		}
	}
	return nil
}

// checkRoomFree: una ubicación activa que nombra una sala debe ser la única activa de su tenant que la nombre.
func (m *Module) checkRoomFree(l SupplyLocation) error {
	if l.RoomId == "" || !l.IsActive {
		return nil
	}
	_, found, err := m.locationOfRoom(l.TenantId, l.RoomId, l.Id)
	if err != nil {
		return err
	}
	if found {
		return ErrRoomTaken
	}
	return nil
}

// locationOfRoom devuelve la ubicación activa del tenant que abastece a roomId, omitiendo la ubicación exceptId.
func (m *Module) locationOfRoom(tenantId, roomId, exceptId string) (SupplyLocation, bool, error) {
	var proxy SupplyLocation
	qb := m.db.Query(&proxy).Where(SupplyLocation_.TenantId).Eq(tenantId).Where(SupplyLocation_.RoomId).Eq(roomId)
	list, err := ReadAllSupplyLocation(qb)
	if err != nil {
		return SupplyLocation{}, false, err
	}
	for _, l := range list {
		if l.IsActive && l.Id != exceptId {
			return *l, true, nil
		}
	}
	return SupplyLocation{}, false, nil
}

// UseInRoom saca la cantidad de un artículo de la ubicación que abastece a roomId, como un consumo (stock.InternalUse)
// causado por el documento (docType, docId) — una atención, para clinical_encounter — por primero en vencer, primero en salir, exactamente
// como RegisterOutput. Devuelve el costo de lo que se tomó en pesos enteros (el costo unitario de cada lote × su cantidad,
// redondeado a medias lejos de cero; 0 cuando los lotes no tienen costo). Sus argumentos son tipos simples para que otro módulo pueda
// declararlo en una interfaz propia sin importar este.
//
// Errores: ErrNoLocationForRoom (ninguna ubicación activa abastece la sala) y cada error de RegisterOutput
// (ErrInsufficientStock, ErrItemNotFound, …). Si el hecho de costo no se puede publicar después del commit, el costo se devuelve
// CON el error: el cambio de stock ya está guardado.
//
// Ejemplo:
//  cost, err := m.UseInRoom("t1", "r1", "1", 2, "clinical_encounter", "doc1", "user1")
func (m *Module) UseInRoom(tenantId, roomId, itemId string, quantity float64, docType, docId, by string) (int64, error) {
	loc, found, err := m.locationOfRoom(tenantId, roomId, "")
	if err != nil {
		return 0, err
	}
	if !found {
		return 0, ErrNoLocationForRoom
	}
	movements, outErr := m.RegisterOutput(RegisterOutputArgs{
		TenantId: tenantId, ItemId: itemId, LocationId: loc.Id, Quantity: quantity, MovementType: string(stock.InternalUse),
		SourceDocumentType: docType, SourceDocumentId: docId, CreatedBy: by,
	})
	if len(movements) == 0 {
		return 0, outErr
	}
	var cost int64
	for _, mv := range movements {
		batch, err := m.GetBatch(tenantId, mv.BatchId)
		if err != nil {
			return cost, err
		}
		cost += int64(float64(batch.UnitCost)*mv.Quantity + 0.5) // half away from zero; both factors are positive
	}
	return cost, outErr
}
