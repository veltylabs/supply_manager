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

// TopicMovementRegistered is published once per stock movement, after the transaction commits. Payload: *SupplyMovement.
const TopicMovementRegistered = "supply.movement.registered"

const (
	// sourceType is the Ref.Type of the cost facts: the movement is their cause.
	sourceType = "supply_movement"

	msgNeedIDs     = "supply: Deps.IDs is required"
	msgPublishCost = "supply: the stock change was saved but its cost fact could not be published:"
)

// Deps are what the module needs from the application that composes it.
type Deps struct {
	IDs        model.IDGenerator // required: the module never builds a generator
	Publisher  events.Publisher  // optional: nil disables the events and the cost facts
	MoneyScope string            // optional: the area of the business the cost facts belong to; "" disables the cost facts
	AlertArea  string            // optional: the application's notify.Area code for this module's alerts; "" disables them
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

func (m *Module) ModelName() string { return ModelName }

// ---- items and locations (the pattern the rest of the CRUD follows) ----

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

// checkItem applies the rules of an item that the field constraints cannot express.
func checkItem(p *SupplyItem) error {
	switch p.Unit {
	case UnitPiece, UnitBox, UnitPair, UnitMl, UnitGram:
	default:
		return ErrUnit
	}
	return nil
}

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

// ---- batches and stock levels ----

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

// levelAt is the stock level item of a batch in a location; found is false when the batch was never there.
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

// levelOrCreate returns the level item of a batch in a location, creating it at quantity 0 and revision 0.
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

// applyToLevel changes the quantity of a stock level by `quantity` in direction d, guarded by the revision the caller
// read: if the item changed since (expectedRevision differs from the stored one) it is ErrConflict and nothing is
// written. stock.Apply keeps the quantity from going negative (ErrInsufficientStock).
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

// addMovement appends one ledger entry inside the transaction.
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

// lotsAt turns the stock of an item in a location into the lots webtyp.com/stock allocates from: one Lot per batch
// that has a level item there, with the batch's expiry and the level's quantity.
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

// ReceiveStock adds stock of an item to a location: it finds or creates the batch (the expiry date is optional, 0 = none), adds
// the quantity to the (location, batch) level and appends a stock.Purchase movement, all in one transaction.
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

// RegisterEntry adds to an EXISTING batch: an inbound correction of a count (stock.AdjustIn).
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

// RegisterOutput takes stock out: an internal use (stock.InternalUse, the use in a consultation) or a write-off (stock.AdjustOut). Without a batch id the batches are drawn by
// first expiry first out (stock.Allocate); only a write-off (stock.AdjustOut) may take expired stock. It returns one
// movement per batch used. If the cost fact cannot be published after the commit, the movements are returned WITH the
// error: the stock change is already saved.
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

// TransferStock moves stock of an item between two locations, drawing the batches by first expiry first out and
// putting the same batches in the destination: stock.TransferOut movements at the origin, stock.TransferIn at the
// destination, one transaction.
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

// publishCosts announces what an output consumed, at cost: one money.Consumed fact per movement whose batch has a unit cost.
// It does nothing unless the application gave a Publisher and a MoneyScope.
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

// checkRoomFree: an active location that names a room must be the only active one of its tenant that names it.
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

// locationOfRoom returns the active location of the tenant that supplies roomId, skipping the location exceptId.
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

// UseInRoom takes quantity of an item out of the location that supplies roomId, as a consumption (stock.InternalUse)
// caused by the document (docType, docId) — an attention, for clinical_encounter — by first expiry first out, exactly
// like RegisterOutput. It returns the cost of what was taken in whole pesos (each batch's unit cost × its quantity,
// rounded half away from zero; 0 when the batches have no cost). Its arguments are plain types so that another module can
// declare it in a narrow interface of its own without importing this one.
//
// Errors: ErrNoLocationForRoom (no active location supplies the room) and every error of RegisterOutput
// (ErrInsufficientStock, ErrItemNotFound, …). If the cost fact cannot be published after the commit, the cost is returned
// WITH the error: the stock change is already saved.
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
