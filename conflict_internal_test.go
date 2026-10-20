// Root-level test (justified): exercises the unexported applyToLevel — the public API cannot force a stale revision.

package supplymanager

import (
	"testing"

	"webtyp.com/events/mock"
	"webtyp.com/fmt"
	"webtyp.com/orm"
	"webtyp.com/stock"
	"webtyp.com/storage/mem"
	"webtyp.com/time"
)

type countingIDs struct{ n int }

func (g *countingIDs) NewID() string {
	g.n++
	return "id-" + fmt.Convert(g.n).String()
}

func TestApplyToLevel_StaleRevisionIsAConflict(t *testing.T) {
	m, err := New(orm.New(mem.New()), Deps{IDs: &countingIDs{}, Publisher: &mock.Broker{}})
	if err != nil {
		t.Fatal(err)
	}
	const tenant = "t1"
	p, err := m.CreateItem(SupplyItem{TenantId: tenant, Sku: "p1", Name: "P", Unit: UnitBox, IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	l, err := m.CreateLocation(SupplyLocation{TenantId: tenant, Name: "Store", IsActive: true})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := m.ReceiveStock(ReceiveStockArgs{TenantId: tenant, ItemId: p.Id, LocationId: l.Id, BatchNumber: "L1", ExpirationDate: time.Now() + 30*stock.SecondsPerDay, Quantity: 10}); err != nil {
		t.Fatal(err)
	}

	stocks, err := m.ListStock(ListStockArgs{TenantId: tenant})
	if err != nil || len(stocks) != 1 {
		t.Fatalf("one level expected: %v, %v", stocks, err)
	}
	level := stocks[0]
	read := level.Revision // what a concurrent writer read

	// somebody else changes the row first: the revision moves on
	err = m.db.Tx(func(tx *orm.DB) error { return applyToLevel(tx, &level, read, stock.In, 1, time.Now()) })
	if err != nil {
		t.Fatalf("the first writer must succeed: %v", err)
	}

	// the second writer still holds the old revision
	stale := level
	err = m.db.Tx(func(tx *orm.DB) error { return applyToLevel(tx, &stale, read, stock.In, 1, time.Now()) })
	if err != ErrConflict {
		t.Errorf("a stale revision: got %v, want ErrConflict", err)
	}
	after, _ := m.ListStock(ListStockArgs{TenantId: tenant})
	if len(after) != 1 || after[0].Quantity != 11 {
		t.Errorf("the conflict must not change the quantity (11 expected), got %+v", after)
	}
}
