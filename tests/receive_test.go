package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/stock"
	"webtyp.com/time"
)

// receive builds the arguments of a ReceiveStock for an item and a location; the tests change what they exercise.
func receive(itemId, locationId, batch string, expiry int64, quantity float64) supplymanager.ReceiveStockArgs {
	return supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: itemId, LocationId: locationId, BatchNumber: batch, ExpirationDate: expiry, Quantity: quantity,
	}
}

func TestReceiveStock_AddsUpAndWritesTheLedger(t *testing.T) {
	m := newModule(t)
	p, loc := newItem(t, m, "p1"), newLocation(t, m, "Store")
	future := time.Now() + 30*stock.SecondsPerDay

	first, err := m.ReceiveStock(receive(p.Id, loc.Id, "L1", future, 10))
	if err != nil {
		t.Fatalf("first receive: %v", err)
	}
	if _, err := m.ReceiveStock(receive(p.Id, loc.Id, "L1", future, 5)); err != nil {
		t.Fatalf("second receive into the same batch: %v", err)
	}

	levels, err := m.ListStock(supplymanager.ListStockArgs{TenantId: testTenant, ItemId: p.Id})
	if err != nil || len(levels) != 1 || levels[0].Quantity != 15 {
		t.Fatalf("one level of 15 expected, got %v, %v", levels, err)
	}
	if first.MovementType != string(stock.Purchase) || first.Quantity != 10 {
		t.Errorf("the first movement must be a PURCHASE of 10, got %+v", first)
	}
}

func TestReceiveStock_Rules(t *testing.T) {
	m := newModule(t)
	p, loc := newItem(t, m, "p1"), newLocation(t, m, "Store")
	future := time.Now() + 30*stock.SecondsPerDay

	cases := []struct {
		name string
		args supplymanager.ReceiveStockArgs
		want error
	}{
		{"negative expiry date", receive(p.Id, loc.Id, "L1", -1, 1), supplymanager.ErrExpiry},
		{"no batch number", receive(p.Id, loc.Id, "", future, 1), supplymanager.ErrBatchNumber},
		{"unknown item", receive("nope", loc.Id, "L1", future, 1), supplymanager.ErrItemNotFound},
		{"unknown location", receive(p.Id, "nope", "L1", future, 1), supplymanager.ErrLocationNotFound},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			if _, err := m.ReceiveStock(c.args); err != c.want {
				t.Errorf("got %v, want %v", err, c.want)
			}
		})
	}

	if _, err := m.ReceiveStock(receive(p.Id, loc.Id, "L9", future, 3)); err != nil {
		t.Fatal(err)
	}
	if _, err := m.ReceiveStock(receive(p.Id, loc.Id, "L9", future+1, 3)); err != supplymanager.ErrBatchExpiryMismatch {
		t.Errorf("the same batch with another expiry: got %v, want ErrBatchExpiryMismatch", err)
	}
}

func TestReceiveStock_AnItemWithoutLotOrExpiry(t *testing.T) {
	m := newModule(t)
	item, loc := newItem(t, m, "gloves"), newLocation(t, m, "Store")

	mv, err := m.ReceiveStock(receive(item.Id, loc.Id, supplymanager.NoBatchNumber, 0, 50))
	if err != nil {
		t.Fatalf("a supply without lot and without expiry must be accepted: %v", err)
	}
	if _, err := m.ReceiveStock(receive(item.Id, loc.Id, supplymanager.NoBatchNumber, 0, 25)); err != nil {
		t.Fatalf("the NO-BATCH batch adds up: %v", err)
	}
	levels, _ := m.ListStock(supplymanager.ListStockArgs{TenantId: testTenant, ItemId: item.Id})
	if len(levels) != 1 || levels[0].Quantity != 75 || levels[0].BatchId != mv.BatchId {
		t.Errorf("one level of 75 in one batch expected, got %v", levels)
	}
}
