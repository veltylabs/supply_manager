package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
)

func TestTenantIsolation(t *testing.T) {
	m := newModule(t)
	item := newItem(t, m, "i1")
	loc := newLocation(t, m, "Loc")

	rec, err := m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "L1", Quantity: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	tenant2 := "tenant-2"

	items, _ := m.ListItems(supplymanager.ListItemsArgs{TenantId: tenant2})
	if len(items) != 0 {
		t.Errorf("expected empty items list, got %v", len(items))
	}

	stocks, _ := m.ListStock(supplymanager.ListStockArgs{TenantId: tenant2, ItemId: item.Id})
	if len(stocks) != 0 {
		t.Errorf("expected empty stock list, got %v", len(stocks))
	}

	_, err = m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: tenant2, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "L2", Quantity: 5,
	})
	if err != supplymanager.ErrItemNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}

	_, err = m.GetBatch(tenant2, rec.BatchId)
	if err != supplymanager.ErrBatchNotFound {
		t.Errorf("expected ErrBatchNotFound, got %v", err)
	}
}
