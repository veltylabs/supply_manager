package tests

import (
	"github.com/veltylabs/supply_manager"
	"testing"
	"webtyp.com/events"
	"webtyp.com/events/mock"
	"webtyp.com/money"
	"webtyp.com/orm"
	"webtyp.com/stock"
	"webtyp.com/storage/mem"
)

func TestEvents_OnePerMovement(t *testing.T) {
	broker := &mock.Broker{}
	m, err := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{IDs: &testIDGen{}, Publisher: broker})
	if err != nil {
		t.Fatal(err)
	}

	n := 0
	broker.Subscribe(supplymanager.TopicMovementRegistered, func(events.Event) { n++ })

	item := newItem(t, m, "i1")
	locA := newLocation(t, m, "A")
	locB := newLocation(t, m, "B")

	_, _ = m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: locA.Id, BatchNumber: "B1", Quantity: 5,
	})
	_, _ = m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: locA.Id, BatchNumber: "B2", Quantity: 5,
	})

	if n != 2 {
		t.Errorf("expected 2 events, got %v", n)
	}

	_, _ = m.RegisterOutput(supplymanager.RegisterOutputArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: locA.Id, Quantity: 7, MovementType: string(stock.InternalUse),
	})

	if n != 4 {
		t.Errorf("expected 4 events, got %v", n)
	}

	_, _ = m.TransferStock(supplymanager.TransferStockArgs{
		TenantId: testTenant, ItemId: item.Id, FromLocationId: locA.Id, ToLocationId: locB.Id, Quantity: 1,
	})

	if n != 6 {
		t.Errorf("expected 6 events, got %v", n)
	}
}

func TestEvents_NilPublisherIsSilent(t *testing.T) {
	m := newModule(t) // no publisher
	item := newItem(t, m, "i1")
	loc := newLocation(t, m, "Loc")

	_, err := m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "B1", Quantity: 5,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = m.RegisterOutput(supplymanager.RegisterOutputArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, Quantity: 2, MovementType: string(stock.InternalUse),
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestEvents_FailedOperationPublishesNothing(t *testing.T) {
	broker := &mock.Broker{}
	m, err := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{IDs: &testIDGen{}, Publisher: broker})
	if err != nil {
		t.Fatal(err)
	}

	n := 0
	broker.Subscribe(supplymanager.TopicMovementRegistered, func(events.Event) { n++ })

	item := newItem(t, m, "i1")
	loc := newLocation(t, m, "Loc")

	_, _ = m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "B1", Quantity: 5,
	})
	nStart := n

	_, err = m.RegisterOutput(supplymanager.RegisterOutputArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, Quantity: 6, MovementType: string(stock.InternalUse),
	})
	if err != supplymanager.ErrInsufficientStock {
		t.Errorf("expected ErrInsufficientStock, got %v", err)
	}

	if n != nStart {
		t.Errorf("expected counter did not move, got %v vs %v", n, nStart)
	}
}

func TestCostFacts_OnlyWithScopeAndCost(t *testing.T) {
	broker := &mock.Broker{}
	sink := &recordingSink{}
	if err := money.Subscribe(broker, sink, func(err error) { t.Errorf("money: %v", err) }); err != nil {
		t.Fatal(err)
	}

	m, err := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{IDs: &testIDGen{}, Publisher: broker})
	if err != nil {
		t.Fatal(err)
	}

	item := newItem(t, m, "i1")
	locA := newLocation(t, m, "A")

	// output with MoneyScope: ""
	rec1, _ := m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: locA.Id, BatchNumber: "B1", Quantity: 5, UnitCost: 100,
	})
	_, _ = m.RegisterOutput(supplymanager.RegisterOutputArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: locA.Id, Quantity: 1, MovementType: string(stock.InternalUse),
	})

	// output of a batch with cost 0 and MoneyScope: "x"
	m2, _ := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{IDs: &testIDGen{}, Publisher: broker, MoneyScope: "x"})
	item2 := newItem(t, m2, "i2")
	locC := newLocation(t, m2, "C")
	rec2, _ := m2.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item2.Id, LocationId: locC.Id, BatchNumber: "B2", Quantity: 5, UnitCost: 0,
	})
	_, _ = m2.RegisterOutput(supplymanager.RegisterOutputArgs{
		TenantId: testTenant, ItemId: item2.Id, LocationId: locC.Id, Quantity: 1, MovementType: string(stock.InternalUse),
	})

	// transfer and entry with cost and scope
	m3, _ := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{IDs: &testIDGen{}, Publisher: broker, MoneyScope: "x"})
	item3 := newItem(t, m3, "i3")
	locD := newLocation(t, m3, "D")
	locE := newLocation(t, m3, "E")
	rec3, _ := m3.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item3.Id, LocationId: locD.Id, BatchNumber: "B3", Quantity: 5, UnitCost: 100,
	})
	_, _ = m3.TransferStock(supplymanager.TransferStockArgs{
		TenantId: testTenant, ItemId: item3.Id, FromLocationId: locD.Id, ToLocationId: locE.Id, Quantity: 1,
	})
	_, _ = m3.RegisterEntry(supplymanager.RegisterEntryArgs{
		TenantId: testTenant, ItemId: item3.Id, LocationId: locE.Id, BatchId: rec3.BatchId, Quantity: 1,
	})

	if len(sink.facts) != 0 {
		t.Errorf("expected no facts, got %v", sink.facts)
	}

	_ = rec1
	_ = rec2
}
