package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/events/mock"
	"webtyp.com/money"
	"webtyp.com/orm"
	"webtyp.com/stock"
	"webtyp.com/storage/mem"
	"webtyp.com/time"
)

// output builds the arguments of a RegisterOutput.
func output(itemId, locationId string, t stock.Movement, quantity float64) supplymanager.RegisterOutputArgs {
	return supplymanager.RegisterOutputArgs{
		TenantId: testTenant, ItemId: itemId, LocationId: locationId, Quantity: quantity, MovementType: string(t),
	}
}

func TestRegisterOutput_DrawsTheEarliestExpiryFirstAndSplits(t *testing.T) {
	m := newModule(t)
	p, loc := newItem(t, m, "p1"), newLocation(t, m, "Store")
	soon, later := time.Now()+10*stock.SecondsPerDay, time.Now()+60*stock.SecondsPerDay
	for _, r := range []supplymanager.ReceiveStockArgs{receive(p.Id, loc.Id, "LATER", later, 10), receive(p.Id, loc.Id, "SOON", soon, 4)} {
		if _, err := m.ReceiveStock(r); err != nil {
			t.Fatal(err)
		}
	}

	mvs, err := m.RegisterOutput(output(p.Id, loc.Id, stock.InternalUse, 6))
	if err != nil {
		t.Fatalf("output: %v", err)
	}
	if len(mvs) != 2 || mvs[0].Quantity != 4 || mvs[1].Quantity != 2 {
		t.Fatalf("SOON (4) must be drained first, then 2 from LATER; got %+v", mvs)
	}
	levels, _ := m.ListStock(supplymanager.ListStockArgs{TenantId: testTenant, ItemId: p.Id})
	if len(levels) != 1 || levels[0].Quantity != 8 {
		t.Errorf("8 must remain in LATER, got %+v", levels)
	}
}

func TestRegisterOutput_Rules(t *testing.T) {
	m := newModule(t)
	p, loc := newItem(t, m, "p1"), newLocation(t, m, "Store")
	expired := time.Now() - 10*stock.SecondsPerDay
	future := time.Now() + 30*stock.SecondsPerDay
	for _, r := range []supplymanager.ReceiveStockArgs{
		receive(p.Id, loc.Id, "OLD", expired, 5), receive(p.Id, loc.Id, "NEW", future, 5),
	} {
		if _, err := m.ReceiveStock(r); err != nil {
			t.Fatal(err)
		}
	}
	oldBatch := batchIDOf(t, m, p.Id, "OLD")

	if _, err := m.RegisterOutput(output(p.Id, loc.Id, stock.Sale, 1)); err != supplymanager.ErrMovementType {
		t.Errorf("supplies are not sold: got %v, want ErrMovementType", err)
	}
	if _, err := m.RegisterOutput(output(p.Id, loc.Id, stock.InternalUse, 6)); err != supplymanager.ErrInsufficientStock {
		t.Errorf("only 5 usable (the expired batch does not count): got %v, want ErrInsufficientStock", err)
	}
	if levels, _ := m.ListStock(supplymanager.ListStockArgs{TenantId: testTenant, ItemId: p.Id}); len(levels) != 2 {
		t.Errorf("a failed output must change nothing, got %+v", levels)
	}
	forced := output(p.Id, loc.Id, stock.InternalUse, 1)
	forced.BatchId = oldBatch
	if _, err := m.RegisterOutput(forced); err != supplymanager.ErrBatchExpired {
		t.Errorf("forcing an expired batch: got %v, want ErrBatchExpired", err)
	}
	forced.MovementType = string(stock.AdjustOut)
	if _, err := m.RegisterOutput(forced); err != nil {
		t.Errorf("a write-off may take expired stock: %v", err)
	}
	if _, err := m.RegisterOutput(output(p.Id, loc.Id, stock.Purchase, 1)); err != supplymanager.ErrMovementType {
		t.Errorf("an inbound type in an output: got %v, want ErrMovementType", err)
	}
}

func batchIDOf(t *testing.T, m *supplymanager.Module, itemId, number string) string {
	t.Helper()
	batches, err := m.ListBatches(testTenant, itemId)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range batches {
		if b.BatchNumber == number {
			return b.Id
		}
	}
	t.Fatalf("batch %s not found", number)
	return ""
}

// recordingSink is a money.Sink that keeps what it accepts.
type recordingSink struct{ facts []money.Fact }

func (s *recordingSink) Accept(f money.Fact) error { s.facts = append(s.facts, f); return nil }

func TestRegisterOutput_PublishesTheCostAtCost(t *testing.T) {
	broker := &mock.Broker{}
	sink := &recordingSink{}
	if err := money.Subscribe(broker, sink, func(err error) { t.Errorf("money: %v", err) }); err != nil {
		t.Fatal(err)
	}
	m, err := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{IDs: &testIDGen{}, Publisher: broker, MoneyScope: "supply"})
	if err != nil {
		t.Fatal(err)
	}
	p, loc := newItem(t, m, "p1"), newLocation(t, m, "Store")
	args := receive(p.Id, loc.Id, "L1", time.Now()+30*stock.SecondsPerDay, 10)
	args.UnitCost = 333
	if _, err := m.ReceiveStock(args); err != nil {
		t.Fatal(err)
	}
	if len(sink.facts) != 0 {
		t.Fatalf("a purchase is not a fact of this module, got %+v", sink.facts)
	}

	if _, err := m.RegisterOutput(output(p.Id, loc.Id, stock.InternalUse, 2.5)); err != nil {
		t.Fatal(err)
	}
	if len(sink.facts) != 1 {
		t.Fatalf("one Consumed fact expected, got %+v", sink.facts)
	}
	f := sink.facts[0]
	if f.Kind != money.Consumed || f.Side != money.Outflow || f.Scope != "supply" || f.Amount.Amount != 833 || f.Amount.Currency != money.CLP {
		t.Errorf("333 × 2.5 = 832.5 must round to 833 CLP, got %+v", f)
	}
}
