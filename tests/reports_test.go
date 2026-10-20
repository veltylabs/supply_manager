package tests

import (
	"testing"

	"webtyp.com/time"

	supplymanager "github.com/veltylabs/supply_manager"
)

func TestListExpiring(t *testing.T) {
	m := newModule(t)
	p, loc := newItem(t, m, "p1"), newLocation(t, m, "Store")
	now := time.NowUnix()
	for _, r := range []supplymanager.ReceiveStockArgs{
		receive(p.Id, loc.Id, "EXPIRED", now-5*time.Day, 1),
		receive(p.Id, loc.Id, "SOON", now+10*time.Day, 1),
		receive(p.Id, loc.Id, "FAR", now+400*time.Day, 1),
	} {
		if _, err := m.ReceiveStock(r); err != nil {
			t.Fatal(err)
		}
	}

	got, err := m.ListExpiring(supplymanager.ListExpiringArgs{TenantId: testTenant, WithinDays: 30})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0].BatchNumber != "EXPIRED" || !got[0].Expired || got[1].BatchNumber != "SOON" || got[1].Expired {
		t.Fatalf("EXPIRED then SOON expected (FAR excluded), got %+v", got)
	}
	if _, err := m.ListExpiring(supplymanager.ListExpiringArgs{TenantId: testTenant, WithinDays: -1}); err != supplymanager.ErrDays {
		t.Errorf("negative days: got %v, want ErrDays", err)
	}
}

func TestListLowStock(t *testing.T) {
	m := newModule(t)
	store, counter := newLocation(t, m, "Store"), newLocation(t, m, "Counter")
	future := time.NowUnix() + 30*time.Day

	newItem(t, m, "nomin") // an item without minimum: never low
	if _, err := m.CreateItem(supplymanager.SupplyItem{
		TenantId: testTenant, Sku: "withmin", Name: "With minimum", Unit: supplymanager.UnitBox, IsActive: true, MinQuantity: 10,
	}); err != nil {
		t.Fatal(err)
	}
	withMin, _ := m.ListItems(supplymanager.ListItemsArgs{TenantId: testTenant})
	var minID string
	for _, p := range withMin {
		if p.Sku == "withmin" {
			minID = p.Id
		}
	}
	for _, r := range []supplymanager.ReceiveStockArgs{receive(minID, store.Id, "A", future, 4), receive(minID, counter.Id, "B", future, 6)} {
		if _, err := m.ReceiveStock(r); err != nil {
			t.Fatal(err)
		}
	}

	got, err := m.ListLowStock(supplymanager.ListLowStockArgs{TenantId: testTenant})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 1 || got[0].ItemId != minID || got[0].Total != 10 || got[0].MinQuantity != 10 {
		t.Fatalf("the item at exactly its minimum (4 + 6 across two locations = 10) is low; the one without minimum is not; got %+v", got)
	}
}
