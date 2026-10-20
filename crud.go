package supplymanager

import (
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/time"
)

// The operations that the generic view drives (save, update by ids and fields, delete by ids) carry no tenant on the wire:
// the records carry theirs, and an id alone names one row, so these helpers resolve a row by id and then work with the
// tenant the row itself stores. Everything that takes a tenant argument elsewhere in this module filters by it.

// itemByID returns an item whatever its tenant, so that an operation sent by id alone can find its tenant.
func (m *Module) itemByID(id string) (SupplyItem, error) {
	var p SupplyItem
	got, err := ReadOneSupplyItem(m.db.Query(&p).Where(SupplyItem_.Id).Eq(id), &p)
	if orm.IsNotFound(err) {
		return SupplyItem{}, ErrItemNotFound
	}
	if err != nil {
		return SupplyItem{}, err
	}
	return *got, nil
}

func (m *Module) locationByID(id string) (SupplyLocation, error) {
	var l SupplyLocation
	got, err := ReadOneSupplyLocation(m.db.Query(&l).Where(SupplyLocation_.Id).Eq(id), &l)
	if orm.IsNotFound(err) {
		return SupplyLocation{}, ErrLocationNotFound
	}
	if err != nil {
		return SupplyLocation{}, err
	}
	return *got, nil
}

// UpdateItem replaces an item with the record given: same id and tenant, every field validated again.
func (m *Module) UpdateItem(p SupplyItem) (SupplyItem, error) {
	existing, err := getItem(m.db, p.TenantId, p.Id)
	if err != nil {
		return SupplyItem{}, err
	}
	p.UpdatedAt = time.NowNano()
	if err := checkItem(&p); err != nil {
		return SupplyItem{}, err
	}
	if err := p.Validate(model.ActionUpdate); err != nil {
		return SupplyItem{}, ValidationError{Err: err}
	}
	if p.Sku != existing.Sku {
		if _, found, err := m.itemBySKU(m.db, p.TenantId, p.Sku); err != nil {
			return SupplyItem{}, err
		} else if found {
			return SupplyItem{}, ErrSkuExists
		}
	}
	if err := m.db.Update(&p, orm.Eq(SupplyItem_.Id, p.Id), orm.Eq(SupplyItem_.TenantId, p.TenantId)); err != nil {
		return SupplyItem{}, err
	}
	return p, nil
}

// UpsertItem creates the item when it has no id or the id is unknown, otherwise updates it.
func (m *Module) UpsertItem(p SupplyItem) (SupplyItem, error) {
	if p.Id == "" {
		return m.CreateItem(p)
	}
	if _, err := getItem(m.db, p.TenantId, p.Id); err == ErrItemNotFound {
		return m.CreateItem(p)
	} else if err != nil {
		return SupplyItem{}, err
	}
	return m.UpdateItem(p)
}

// PatchItems applies to the items named by ids only the fields named by fields, taking the values from rec: the
// shape of the view's "update" operation. A field this module does not know is a ValidationError.
func (m *Module) PatchItems(ids, fields []string, rec SupplyItem) error {
	for _, id := range ids {
		current, err := m.itemByID(id)
		if err != nil {
			return err
		}
		for _, f := range fields {
			switch f {
			case "sku":
				current.Sku = rec.Sku
			case "name":
				current.Name = rec.Name
			case "unit":
				current.Unit = rec.Unit
			case "min_quantity":
				current.MinQuantity = rec.MinQuantity
			case "is_active":
				current.IsActive = rec.IsActive
			default:
				return ValidationError{Err: ErrUnknownField}
			}
		}
		if _, err := m.UpdateItem(current); err != nil {
			return err
		}
	}
	return nil
}

// DeactivateItem hides an item from new receipts without touching its history.
func (m *Module) DeactivateItem(tenantId, id string) error {
	p, err := getItem(m.db, tenantId, id)
	if err != nil {
		return err
	}
	p.IsActive = false
	p.UpdatedAt = time.NowNano()
	return m.db.Update(&p, orm.Eq(SupplyItem_.Id, p.Id), orm.Eq(SupplyItem_.TenantId, p.TenantId))
}

// DeleteItems removes items by id, each only if it has no batch (ErrItemInUse): the ledger keeps its history.
func (m *Module) DeleteItems(ids []string) error {
	for _, id := range ids {
		p, err := m.itemByID(id)
		if err != nil {
			return err
		}
		batches, err := m.ListBatches(p.TenantId, p.Id)
		if err != nil {
			return err
		}
		if len(batches) > 0 {
			return ErrItemInUse
		}
		if err := m.db.Delete(&p, orm.Eq(SupplyItem_.Id, p.Id), orm.Eq(SupplyItem_.TenantId, p.TenantId)); err != nil {
			return err
		}
	}
	return nil
}

// UpdateLocation replaces a location with the record given.
func (m *Module) UpdateLocation(l SupplyLocation) (SupplyLocation, error) {
	existing, err := getLocation(m.db, l.TenantId, l.Id)
	if err != nil {
		return SupplyLocation{}, err
	}
	l.UpdatedAt = time.NowNano()
	if err := l.Validate(model.ActionUpdate); err != nil {
		return SupplyLocation{}, ValidationError{Err: err}
	}
	if l.Name != existing.Name {
		var proxy SupplyLocation
		qb := m.db.Query(&proxy).Where(SupplyLocation_.Name).Eq(l.Name).Where(SupplyLocation_.TenantId).Eq(l.TenantId)
		if _, err := ReadOneSupplyLocation(qb, &proxy); err == nil {
			return SupplyLocation{}, ErrLocationNameExists
		} else if !orm.IsNotFound(err) {
			return SupplyLocation{}, err
		}
	}
	if err := m.checkRoomFree(l); err != nil {
		return SupplyLocation{}, err
	}
	if err := m.db.Update(&l, orm.Eq(SupplyLocation_.Id, l.Id), orm.Eq(SupplyLocation_.TenantId, l.TenantId)); err != nil {
		return SupplyLocation{}, err
	}
	return l, nil
}

// UpsertLocation creates the location when it has no id or the id is unknown, otherwise updates it.
func (m *Module) UpsertLocation(l SupplyLocation) (SupplyLocation, error) {
	if l.Id == "" {
		return m.CreateLocation(l)
	}
	if _, err := getLocation(m.db, l.TenantId, l.Id); err == ErrLocationNotFound {
		return m.CreateLocation(l)
	} else if err != nil {
		return SupplyLocation{}, err
	}
	return m.UpdateLocation(l)
}

// PatchLocations applies to the locations named by ids only the fields named by fields, taking the values from rec.
func (m *Module) PatchLocations(ids, fields []string, rec SupplyLocation) error {
	for _, id := range ids {
		current, err := m.locationByID(id)
		if err != nil {
			return err
		}
		for _, f := range fields {
			switch f {
			case "name":
				current.Name = rec.Name
			case "room_id":
				current.RoomId = rec.RoomId
			case "is_active":
				current.IsActive = rec.IsActive
			default:
				return ValidationError{Err: ErrUnknownField}
			}
		}
		if _, err := m.UpdateLocation(current); err != nil {
			return err
		}
	}
	return nil
}

// DeleteLocations removes locations by id, each only if no stock level row references it (ErrLocationInUse).
func (m *Module) DeleteLocations(ids []string) error {
	for _, id := range ids {
		l, err := m.locationByID(id)
		if err != nil {
			return err
		}
		var proxy SupplyStockLevel
		levels, err := ReadAllSupplyStockLevel(m.db.Query(&proxy).Where(SupplyStockLevel_.LocationId).Eq(l.Id).Where(SupplyStockLevel_.TenantId).Eq(l.TenantId))
		if err != nil {
			return err
		}
		if len(levels) > 0 {
			return ErrLocationInUse
		}
		if err := m.db.Delete(&l, orm.Eq(SupplyLocation_.Id, l.Id), orm.Eq(SupplyLocation_.TenantId, l.TenantId)); err != nil {
			return err
		}
	}
	return nil
}
