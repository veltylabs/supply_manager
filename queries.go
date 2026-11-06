package supplymanager

import (
	"slices"
)

// defaultMovementLimit es el número de movimientos que devuelve list_movements cuando el llamador no especifica un límite.
const defaultMovementLimit = 100

// ListBatches devuelve los lotes de un artículo.
//
// Ejemplo:
//  batches, err := m.ListBatches("t1", "1")
func (m *Module) ListBatches(tenantId, itemId string) ([]SupplyBatch, error) {
	var proxy SupplyBatch
	qb := m.db.Query(&proxy).Where(SupplyBatch_.TenantId).Eq(tenantId).Where(SupplyBatch_.ItemId).Eq(itemId)
	list, err := ReadAllSupplyBatch(qb)
	if err != nil {
		return nil, err
	}
	return derefBatches(list), nil
}

func derefBatches(list SupplyBatchList) []SupplyBatch {
	out := make([]SupplyBatch, len(list))
	for i, b := range list {
		out[i] = *b
	}
	return out
}

// batchIDsOf devuelve los ids de los lotes de un artículo.
func (m *Module) batchIDsOf(tenantId, itemId string) ([]string, error) {
	batches, err := m.ListBatches(tenantId, itemId)
	if err != nil {
		return nil, err
	}
	ids := make([]string, len(batches))
	for i, b := range batches {
		ids[i] = b.Id
	}
	return ids, nil
}

// ListStock devuelve los niveles de stock con cantidad > 0, opcionalmente de un artículo y/o una ubicación.
//
// Ejemplo:
//  stock, err := m.ListStock(ListStockArgs{TenantId: "t1", ItemId: "1"})
func (m *Module) ListStock(a ListStockArgs) ([]SupplyStockLevel, error) {
	var proxy SupplyStockLevel
	qb := m.db.Query(&proxy).Where(SupplyStockLevel_.TenantId).Eq(a.TenantId)
	if a.LocationId != "" {
		qb = qb.Where(SupplyStockLevel_.LocationId).Eq(a.LocationId)
	}
	list, err := ReadAllSupplyStockLevel(qb)
	if err != nil {
		return nil, err
	}
	var batchIDs []string
	if a.ItemId != "" {
		if batchIDs, err = m.batchIDsOf(a.TenantId, a.ItemId); err != nil {
			return nil, err
		}
	}
	var out []SupplyStockLevel
	for _, l := range list {
		if l.Quantity <= 0 {
			continue
		}
		if a.ItemId != "" && !slices.Contains(batchIDs, l.BatchId) {
			continue
		}
		out = append(out, *l)
	}
	return out, nil
}

// ListMovements devuelve el libro mayor, desde el más reciente, opcionalmente de un artículo o un lote y entre dos segundos Unix
// (un límite de cero significa abierto). Sin un límite, devuelve defaultMovementLimit entradas.
//
// Ejemplo:
//  movs, err := m.ListMovements(ListMovementsArgs{TenantId: "t1", ItemId: "1", Limit: 10})
func (m *Module) ListMovements(a ListMovementsArgs) ([]SupplyMovement, error) {
	var proxy SupplyMovement
	qb := m.db.Query(&proxy).Where(SupplyMovement_.TenantId).Eq(a.TenantId)
	if a.BatchId != "" {
		qb = qb.Where(SupplyMovement_.BatchId).Eq(a.BatchId)
	}
	list, err := ReadAllSupplyMovement(qb)
	if err != nil {
		return nil, err
	}
	var batchIDs []string
	if a.ItemId != "" {
		if batchIDs, err = m.batchIDsOf(a.TenantId, a.ItemId); err != nil {
			return nil, err
		}
	}
	var out []SupplyMovement
	for _, mv := range list {
		if a.ItemId != "" && !slices.Contains(batchIDs, mv.BatchId) {
			continue
		}
		if (a.From > 0 && mv.CreatedAt < a.From) || (a.To > 0 && mv.CreatedAt > a.To) {
			continue
		}
		out = append(out, *mv)
	}
	slices.Reverse(out) // el almacén devuelve el orden de inserción: al revés, sirve como desempate para entradas escritas en el mismo segundo
	slices.SortStableFunc(out, func(x, y SupplyMovement) int {
		switch {
		case x.CreatedAt > y.CreatedAt:
			return -1
		case x.CreatedAt < y.CreatedAt:
			return 1
		}
		return 0
	})
	limit := int(a.Limit)
	if limit <= 0 {
		limit = defaultMovementLimit
	}
	if len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

// GetBatch devuelve un lote del tenant.
//
// Ejemplo:
//  batch, err := m.GetBatch("t1", "1")
func (m *Module) GetBatch(tenantId, id string) (SupplyBatch, error) {
	return getBatch(m.db, tenantId, id)
}

// ListItems devuelve los artículos del tenant del módulo, buscando por nombre y filtrando por página.
//
// Ejemplo:
//  items, err := m.ListItems("term", "", 50)
func (m *Module) ListItems(search, after string, limit int64) ([]SupplyItem, error) {
	var proxy SupplyItem
	qb := m.db.Query(&proxy).Where(SupplyItem_.TenantId).Eq(m.tenantID)
	if search != "" {
		qb = qb.Where(SupplyItem_.Name).Like("%" + search + "%")
	}
	if after != "" {
		qb = qb.Where(SupplyItem_.Id).Gt(after)
	}
	qb = qb.OrderBy(SupplyItem_.Id).Asc()
	if limit > 0 {
		qb = qb.Limit(int(limit))
	}
	list, err := ReadAllSupplyItem(qb)
	if err != nil {
		return nil, err
	}
	out := make([]SupplyItem, len(list))
	for i, p := range list {
		out[i] = *p
	}
	return out, nil
}

// ListLocations devuelve las ubicaciones del tenant del módulo, buscando por nombre y filtrando por página.
//
// Ejemplo:
//  locs, err := m.ListLocations("term", "", 50)
func (m *Module) ListLocations(search, after string, limit int64) ([]SupplyLocation, error) {
	var proxy SupplyLocation
	qb := m.db.Query(&proxy).Where(SupplyLocation_.TenantId).Eq(m.tenantID)
	if search != "" {
		qb = qb.Where(SupplyLocation_.Name).Like("%" + search + "%")
	}
	if after != "" {
		qb = qb.Where(SupplyLocation_.Id).Gt(after)
	}
	qb = qb.OrderBy(SupplyLocation_.Id).Asc()
	if limit > 0 {
		qb = qb.Limit(int(limit))
	}
	list, err := ReadAllSupplyLocation(qb)
	if err != nil {
		return nil, err
	}
	out := make([]SupplyLocation, len(list))
	for i, l := range list {
		out[i] = *l
	}
	return out, nil
}

// compareText ordena dos cadenas por orden de bytes.
func compareText(a, b string) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

// page devuelve el segmento desde offset, con una longitud máxima de limit; limit 0 = hasta el final.
func page[T any](list []T, offset, limit int) []T {
	if offset > 0 {
		if offset >= len(list) {
			return nil
		}
		list = list[offset:]
	}
	if limit > 0 && len(list) > limit {
		list = list[:limit]
	}
	return list
}
