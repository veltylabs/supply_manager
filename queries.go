package supplymanager

import (
	"slices"
)

// defaultMovementLimit is how many movements list_movements returns when the caller gives no limit.
const defaultMovementLimit = 100

// ListBatches returns the batches of an item.
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

// batchIDsOf returns the ids of the batches of an item.
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

// ListStock returns the stock levels with quantity > 0, optionally of one item and/or one location.
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

// ListMovements returns the ledger, newest first, optionally of one item or one batch and between two Unix seconds
// (a zero bound is open). Without a limit it returns defaultMovementLimit entries.
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

// GetBatch returns one batch of the tenant.
func (m *Module) GetBatch(tenantId, id string) (SupplyBatch, error) {
	return getBatch(m.db, tenantId, id)
}
