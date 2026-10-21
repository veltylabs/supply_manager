package tests

import (
	"testing"

	"webtyp.com/events"
	"webtyp.com/events/mock"
	"webtyp.com/notify"
	"webtyp.com/orm"
	"webtyp.com/stock"
	"webtyp.com/storage/mem"
	"webtyp.com/time"

	supplymanager "github.com/veltylabs/supply_manager"
)

func TestAlerts(t *testing.T) {
	t.Run("AlertKinds", func(t *testing.T) {
		kinds := supplymanager.AlertKinds("supplies")
		if len(kinds) != 2 {
			t.Fatalf("expected 2 alert kinds, got %d", len(kinds))
		}
		if kinds[0].Code != supplymanager.AlertBelowMinimum || kinds[0].Area != "supplies" {
			t.Errorf("unexpected first kind: %+v", kinds[0])
		}
		if kinds[1].Code != supplymanager.AlertExpiring || kinds[1].Area != "supplies" {
			t.Errorf("unexpected second kind: %+v", kinds[1])
		}
	})

	t.Run("Below minimum", func(t *testing.T) {
		broker := &mock.Broker{}
		m, err := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{
			IDs:       &testIDGen{},
			Publisher: broker,
			AlertArea: "supplies_area",
		})
		if err != nil {
			t.Fatal(err)
		}

		var alerts []*notify.Alert
		broker.Subscribe(notify.TopicAlert, func(e events.Event) {
			alerts = append(alerts, e.Payload.(*notify.Alert))
		})

		item, err := m.CreateItem(supplymanager.SupplyItem{
			TenantId: testTenant, Sku: "i1", Name: "Item i1", Unit: supplymanager.UnitBox,
			IsActive: true, MinQuantity: 10,
		})
		if err != nil {
			t.Fatal(err)
		}

		loc := newLocation(t, m, "L1")

		// total 12
		_, err = m.ReceiveStock(supplymanager.ReceiveStockArgs{
			TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "B1", Quantity: 12, UnitCost: 0,
		})
		if err != nil {
			t.Fatal(err)
		}

		// output 3 (total -> 9), crosses minimum 10
		_, err = m.RegisterOutput(supplymanager.RegisterOutputArgs{
			TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, Quantity: 3, MovementType: string(stock.InternalUse),
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(alerts))
		}

		if alerts[0].Code != supplymanager.AlertBelowMinimum || alerts[0].Scope != "supplies_area" || alerts[0].Source.ID != item.Id || alerts[0].Body != "9 BOX left; minimum 10." {
			t.Errorf("unexpected alert: %+v", alerts[0])
		}

		// output 1 (total -> 8), already below minimum, no new alert
		_, err = m.RegisterOutput(supplymanager.RegisterOutputArgs{
			TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, Quantity: 1, MovementType: string(stock.InternalUse),
		})
		if err != nil {
			t.Fatal(err)
		}

		if len(alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(alerts))
		}
	})

	t.Run("ReviewExpiring", func(t *testing.T) {
		broker := &mock.Broker{}
		m, err := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{
			IDs:       &testIDGen{},
			Publisher: broker,
			AlertArea: "supplies_area",
		})
		if err != nil {
			t.Fatal(err)
		}

		var alerts []*notify.Alert
		broker.Subscribe(notify.TopicAlert, func(e events.Event) {
			alerts = append(alerts, e.Payload.(*notify.Alert))
		})

		item := newItem(t, m, "i2")
		loc := newLocation(t, m, "L2")

		now := time.NowUnix()
		// One expired 1 day ago
		_, err = m.ReceiveStock(supplymanager.ReceiveStockArgs{
			TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "EXP1", Quantity: 5, ExpirationDate: now - 86400,
		})
		if err != nil {
			t.Fatal(err)
		}

		// One expiring in 5 days
		_, err = m.ReceiveStock(supplymanager.ReceiveStockArgs{
			TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "SOON1", Quantity: 5, ExpirationDate: now + (86400 * 5),
		})
		if err != nil {
			t.Fatal(err)
		}

		// One expiring in 60 days
		_, err = m.ReceiveStock(supplymanager.ReceiveStockArgs{
			TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "FAR1", Quantity: 5, ExpirationDate: now + (86400 * 60),
		})
		if err != nil {
			t.Fatal(err)
		}

		rows, err := m.ReviewExpiring(testTenant, 30)
		if err != nil {
			t.Fatal(err)
		}

		if len(rows) != 2 {
			t.Fatalf("expected 2 expiring rows, got %d", len(rows))
		}

		if len(alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(alerts))
		}

		if alerts[0].Code != supplymanager.AlertExpiring || alerts[0].Scope != "supplies_area" || alerts[0].Body != "1 expired batches and 1 expiring within 30 days." {
			t.Errorf("unexpected alert: %+v", alerts[0])
		}

		// Run again with no expiring
		m2, err := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{
			IDs:       &testIDGen{},
			Publisher: broker,
			AlertArea: "supplies_area",
		})
		if err != nil {
			t.Fatal(err)
		}

		rows2, err := m2.ReviewExpiring(testTenant, 30)
		if err != nil {
			t.Fatal(err)
		}
		if len(rows2) != 0 {
			t.Fatalf("expected 0 rows, got %d", len(rows2))
		}

		// Number of alerts should remain 1
		if len(alerts) != 1 {
			t.Fatalf("expected 1 alert, got %d", len(alerts))
		}
	})
}
