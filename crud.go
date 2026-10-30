package supplymanager

import (
	"webtyp.com/model"
	"webtyp.com/orm"
	"webtyp.com/time"
)

// Las operaciones que maneja la vista genérica (guardar, actualizar por ids y campos, borrar por ids) no llevan tenant en la red:
// los registros llevan el suyo, y un id por sí solo nombra un artículo, así que estos ayudantes resuelven un artículo por id y luego trabajan con el
// tenant que el propio artículo almacena. Todo lo que toma un argumento de tenant en otra parte de este módulo filtra por él.

// itemByID devuelve un artículo sin importar su tenant, para que una operación enviada solo por id pueda encontrar su tenant.
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

// UpdateItem reemplaza un artículo con el registro dado: mismo id y tenant, validando cada campo de nuevo.
//
// Ejemplo:
//  item, err := m.UpdateItem(SupplyItem{TenantId: "t1", Id: "1", Sku: "123", Name: "Guantes", Unit: UnitBox, IsActive: true})
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
		return SupplyItem{}, err
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

// UpsertItem crea el artículo cuando no tiene id o el id es desconocido, de lo contrario lo actualiza.
//
// Ejemplo:
//  item, err := m.UpsertItem(SupplyItem{TenantId: "t1", Sku: "123", Name: "Guantes", Unit: UnitBox, IsActive: true})
func (m *Module) UpsertItem(p SupplyItem) (SupplyItem, error) {
	if p.Id == "" {
		return m.CreateItem(p)
	}
	if _, err := getItem(m.db, p.TenantId, p.Id); model.KindOf(err) == model.ErrorNotFound {
		return m.CreateItem(p)
	} else if err != nil {
		return SupplyItem{}, err
	}
	return m.UpdateItem(p)
}

// PatchItems aplica a los artículos nombrados por ids solo los campos nombrados por fields, tomando los valores de rec: la
// forma de la operación "update" de la vista. Un campo que este módulo no conoce es un ValidationError.
//
// Ejemplo:
//  err := m.PatchItems([]string{"1", "2"}, []string{"is_active"}, SupplyItem{IsActive: false})
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
				return &model.FieldError{Field: f, Err: ErrUnknownField}
			}
		}
		if _, err := m.UpdateItem(current); err != nil {
			return err
		}
	}
	return nil
}

// DeactivateItem oculta un artículo de nuevas recepciones sin tocar su historial.
//
// Ejemplo:
//  err := m.DeactivateItem("t1", "1")
func (m *Module) DeactivateItem(tenantId, id string) error {
	p, err := getItem(m.db, tenantId, id)
	if err != nil {
		return err
	}
	p.IsActive = false
	p.UpdatedAt = time.NowNano()
	return m.db.Update(&p, orm.Eq(SupplyItem_.Id, p.Id), orm.Eq(SupplyItem_.TenantId, p.TenantId))
}

// DeleteItems elimina artículos por id, cada uno solo si no tiene lote (ErrItemInUse): el libro mayor mantiene su historial.
//
// Ejemplo:
//  err := m.DeleteItems([]string{"1", "2"})
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

// UpdateLocation reemplaza una ubicación con el registro dado.
//
// Ejemplo:
//  loc, err := m.UpdateLocation(SupplyLocation{TenantId: "t1", Id: "1", Name: "Depósito", IsActive: true})
func (m *Module) UpdateLocation(l SupplyLocation) (SupplyLocation, error) {
	existing, err := getLocation(m.db, l.TenantId, l.Id)
	if err != nil {
		return SupplyLocation{}, err
	}
	l.UpdatedAt = time.NowNano()
	if err := l.Validate(model.ActionUpdate); err != nil {
		return SupplyLocation{}, err
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

// UpsertLocation crea la ubicación cuando no tiene id o el id es desconocido, de lo contrario la actualiza.
//
// Ejemplo:
//  loc, err := m.UpsertLocation(SupplyLocation{TenantId: "t1", Name: "Depósito", IsActive: true})
func (m *Module) UpsertLocation(l SupplyLocation) (SupplyLocation, error) {
	if l.Id == "" {
		return m.CreateLocation(l)
	}
	if _, err := getLocation(m.db, l.TenantId, l.Id); model.KindOf(err) == model.ErrorNotFound {
		return m.CreateLocation(l)
	} else if err != nil {
		return SupplyLocation{}, err
	}
	return m.UpdateLocation(l)
}

// PatchLocations aplica a las ubicaciones nombradas por ids solo los campos nombrados por fields, tomando los valores de rec.
//
// Ejemplo:
//  err := m.PatchLocations([]string{"1"}, []string{"is_active"}, SupplyLocation{IsActive: false})
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
				return &model.FieldError{Field: f, Err: ErrUnknownField}
			}
		}
		if _, err := m.UpdateLocation(current); err != nil {
			return err
		}
	}
	return nil
}

// DeleteLocations elimina ubicaciones por id, cada una solo si ningún nivel de stock la referencia (ErrLocationInUse).
//
// Ejemplo:
//  err := m.DeleteLocations([]string{"1"})
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
