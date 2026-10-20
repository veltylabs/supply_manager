package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/stock"
)

// roomLocation creates an active location that supplies roomId.
func roomLocation(t *testing.T, m *supplymanager.Module, name, roomId string) supplymanager.SupplyLocation {
	t.Helper()
	l, err := m.CreateLocation(supplymanager.SupplyLocation{TenantId: testTenant, Name: name, RoomId: roomId, IsActive: true})
	if err != nil {
		t.Fatalf("CreateLocation(%s): %v", name, err)
	}
	return l
}

func TestLocation_ARoomIsSuppliedByOneActiveLocation(t *testing.T) {
	m := newModule(t)
	roomLocation(t, m, "Box 1 shelf", "room-1")

	if _, err := m.CreateLocation(supplymanager.SupplyLocation{TenantId: testTenant, Name: "Other shelf", RoomId: "room-1", IsActive: true}); err != supplymanager.ErrRoomTaken {
		t.Errorf("second active location for the same room: got %v, want ErrRoomTaken", err)
	}
	if _, err := m.CreateLocation(supplymanager.SupplyLocation{TenantId: testTenant, Name: "Old shelf", RoomId: "room-1", IsActive: false}); err != nil {
		t.Errorf("an inactive location may name a taken room: %v", err)
	}
	if _, err := m.CreateLocation(supplymanager.SupplyLocation{TenantId: "tenant-2", Name: "Box 1 shelf", RoomId: "room-1", IsActive: true}); err != nil {
		t.Errorf("another tenant's room is another room: %v", err)
	}
	other := newLocation(t, m, "Store")
	other.RoomId = "room-1"
	if _, err := m.UpdateLocation(other); err != supplymanager.ErrRoomTaken {
		t.Errorf("update onto a taken room: got %v, want ErrRoomTaken", err)
	}
}

func TestUseInRoom_TakesFromTheRoomLocationAndReturnsTheCost(t *testing.T) {
	m := newModule(t)
	item := newItem(t, m, "gauze")
	shelf := roomLocation(t, m, "Box 1 shelf", "room-1")
	store := newLocation(t, m, "Store")
	for _, r := range []supplymanager.ReceiveStockArgs{
		{TenantId: testTenant, ItemId: item.Id, LocationId: shelf.Id, BatchNumber: "A", ExpirationDate: nowSec() + 10*stock.SecondsPerDay, UnitCost: 100, Quantity: 2},
		{TenantId: testTenant, ItemId: item.Id, LocationId: shelf.Id, BatchNumber: "B", ExpirationDate: nowSec() + 90*stock.SecondsPerDay, UnitCost: 150, Quantity: 5},
		{TenantId: testTenant, ItemId: item.Id, LocationId: store.Id, BatchNumber: "C", ExpirationDate: nowSec() + 5*stock.SecondsPerDay, UnitCost: 1, Quantity: 50},
	} {
		if _, err := m.ReceiveStock(r); err != nil {
			t.Fatalf("ReceiveStock %s: %v", r.BatchNumber, err)
		}
	}

	cost, err := m.UseInRoom(testTenant, "room-1", item.Id, 3, "encounter", "enc-1", "doc-1")
	if err != nil {
		t.Fatalf("UseInRoom: %v", err)
	}
	if cost != 2*100+1*150 {
		t.Errorf("cost: got %d, want 350 (2 of A at 100, then 1 of B at 150: first expiry first out)", cost)
	}
	levels, err := m.ListStock(supplymanager.ListStockArgs{TenantId: testTenant, ItemId: item.Id, LocationId: store.Id})
	if err != nil || len(levels) != 1 || levels[0].Quantity != 50 {
		t.Errorf("the store must be untouched, got %+v %v", levels, err)
	}
	mvs, err := m.ListMovements(supplymanager.ListMovementsArgs{TenantId: testTenant, ItemId: item.Id})
	if err != nil {
		t.Fatal(err)
	}
	outs := 0
	for _, mv := range mvs {
		if mv.MovementType == string(stock.InternalUse) {
			outs++
			if mv.LocationId != shelf.Id || mv.SourceDocumentType != "encounter" || mv.SourceDocumentId != "enc-1" || mv.CreatedBy != "doc-1" {
				t.Errorf("the use must carry the room location and the document: %+v", mv)
			}
		}
	}
	if outs != 2 {
		t.Errorf("one movement per batch taken: got %d, want 2", outs)
	}
}

func TestUseInRoom_Errors(t *testing.T) {
	m := newModule(t)
	item := newItem(t, m, "gloves")
	shelf := roomLocation(t, m, "Box 1 shelf", "room-1")
	if _, err := m.ReceiveStock(supplymanager.ReceiveStockArgs{TenantId: testTenant, ItemId: item.Id, LocationId: shelf.Id, BatchNumber: supplymanager.NoBatchNumber, Quantity: 1}); err != nil {
		t.Fatal(err)
	}

	if _, err := m.UseInRoom(testTenant, "room-9", item.Id, 1, "encounter", "e", ""); err != supplymanager.ErrNoLocationForRoom {
		t.Errorf("room without location: got %v, want ErrNoLocationForRoom", err)
	}
	if _, err := m.UseInRoom(testTenant, "room-1", item.Id, 2, "encounter", "e", ""); err != supplymanager.ErrInsufficientStock {
		t.Errorf("not enough: got %v, want ErrInsufficientStock", err)
	}
	if cost, err := m.UseInRoom(testTenant, "room-1", item.Id, 1, "encounter", "e", ""); err != nil || cost != 0 {
		t.Errorf("a batch without cost costs 0: got %d %v", cost, err)
	}
	if _, err := m.UseInRoom("tenant-2", "room-1", item.Id, 1, "encounter", "e", ""); err != supplymanager.ErrNoLocationForRoom {
		t.Errorf("another tenant does not see the room's location: got %v", err)
	}
}
