package supplymanager

import (
	"webtyp.com/time"

	"slices"

	"webtyp.com/stock"
)

// ListExpiring returns the stock (quantity > 0) whose batch is expired or expires within WithinDays days, soonest first.
// The rule is stock.Lot.ExpiresWithin; this module does not repeat it.
func (m *Module) ListExpiring(a ListExpiringArgs) ([]ExpiringStock, error) {
	if a.WithinDays < 0 {
		return nil, ErrDays
	}
	now := time.NowUnix()
	levels, err := m.ListStock(ListStockArgs{TenantId: a.TenantId})
	if err != nil {
		return nil, err
	}
	var out []ExpiringStock
	for _, level := range levels {
		batch, err := getBatch(m.db, a.TenantId, level.BatchId)
		if err != nil {
			return nil, err
		}
		lot := stock.Lot{ID: batch.Id, Expiry: batch.ExpirationDate, Quantity: level.Quantity}
		if !lot.ExpiresWithin(now, int(a.WithinDays)) {
			continue
		}
		item, err := getItem(m.db, a.TenantId, batch.ItemId)
		if err != nil {
			return nil, err
		}
		out = append(out, ExpiringStock{
			ItemId: item.Id, ItemName: item.Name, BatchId: batch.Id, BatchNumber: batch.BatchNumber,
			LocationId: level.LocationId, Quantity: level.Quantity, ExpirationDate: batch.ExpirationDate, Expired: lot.Expired(now),
		})
	}
	slices.SortStableFunc(out, func(x, y ExpiringStock) int {
		switch {
		case x.ExpirationDate < y.ExpirationDate:
			return -1
		case x.ExpirationDate > y.ExpirationDate:
			return 1
		}
		return 0
	})
	return out, nil
}

// ListLowStock returns the active items with a minimum quantity (> 0) whose total, over every location and batch, is at
// or below it, by name. A later alert module calls it; this module raises no alert.
func (m *Module) ListLowStock(a ListLowStockArgs) ([]LowStock, error) {
	items, err := m.ListItems(ListItemsArgs{TenantId: a.TenantId, ActiveOnly: true})
	if err != nil {
		return nil, err
	}
	var out []LowStock
	for _, p := range items {
		if p.MinQuantity <= 0 {
			continue
		}
		levels, err := m.ListStock(ListStockArgs{TenantId: a.TenantId, ItemId: p.Id})
		if err != nil {
			return nil, err
		}
		var total float64
		for _, level := range levels {
			total += level.Quantity
		}
		if total <= p.MinQuantity {
			out = append(out, LowStock{ItemId: p.Id, ItemName: p.Name, Total: total, MinQuantity: p.MinQuantity})
		}
	}
	return out, nil
}

// ItemExists tells whether an item exists in the tenant and is active. Other modules declare the same method in a
// narrow interface of their own; *Module satisfies it without importing them.
func (m *Module) ItemExists(tenantId, itemId string) (bool, error) {
	p, err := getItem(m.db, tenantId, itemId)
	if err == ErrItemNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return p.IsActive, nil
}

// ItemCard returns what another module snapshots about an item. An absent or foreign item is ErrNotFound.
func (m *Module) ItemCard(tenantId, itemId string) (name, unit string, active bool, err error) {
	p, err := getItem(m.db, tenantId, itemId)
	if err == ErrItemNotFound {
		return "", "", false, ErrNotFound
	}
	if err != nil {
		return "", "", false, err
	}
	return p.Name, p.Unit, p.IsActive, nil
}
