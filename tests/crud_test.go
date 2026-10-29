package tests

import (
	"webtyp.com/model"
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
)

func TestCreateItem_StoresIt(t *testing.T) {
	m := newModule(t)
	created := newItem(t, m, "a1")
	got, err := m.GetItem(testTenant, created.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if got.Sku != created.Sku || got.Name != created.Name || got.Unit != created.Unit {
		t.Errorf("expected %v %v %v, got %v %v %v", created.Sku, created.Name, created.Unit, got.Sku, got.Name, got.Unit)
	}
	if got.UpdatedAt <= 0 {
		t.Errorf("expected UpdatedAt > 0, got %v", got.UpdatedAt)
	}
}

func TestCreateItem_SkuIsUniquePerTenant(t *testing.T) {
	m := newModule(t)
	_ = newItem(t, m, "a1")

	// Create again in same tenant
	_, err := m.CreateItem(supplymanager.SupplyItem{TenantId: testTenant, Sku: "a1", Name: "Same", Unit: "BOX", IsActive: true})
	if model.KindOf(err) != model.ErrorConflict {
		t.Errorf("expected ErrSkuExists, got %v", err)
	}

	// Create in different tenant
	created2, err2 := m.CreateItem(supplymanager.SupplyItem{TenantId: "tenant-2", Sku: "a1", Name: "Same", Unit: "BOX", IsActive: true})
	if err2 != nil {
		t.Errorf("expected nil error, got %v", err2)
	}
	if created2.Id == "" {
		t.Errorf("expected item to be created in tenant-2")
	}
}

func TestGetItem_ForeignTenant(t *testing.T) {
	m := newModule(t)
	created := newItem(t, m, "a1")

	_, err := m.GetItem("tenant-2", created.Id)
	if model.KindOf(err) != model.ErrorNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestUpdateItem(t *testing.T) {
	m := newModule(t)
	a1 := newItem(t, m, "a1")
	a2 := newItem(t, m, "a2")

	// Update name of a1
	a1.Name = "New Name"
	updated, err := m.UpdateItem(a1)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if updated.Name != "New Name" {
		t.Errorf("expected name to change")
	}

	// Update a1 to have a2's sku
	a1.Sku = a2.Sku
	_, err = m.UpdateItem(a1)
	if model.KindOf(err) != model.ErrorConflict {
		t.Errorf("expected ErrSkuExists, got %v", err)
	}

	// Update unknown id
	unknown := a1
	unknown.Id = "unknown"
	unknown.Sku = "a3"
	_, err = m.UpdateItem(unknown)
	if model.KindOf(err) != model.ErrorNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestPatchItems(t *testing.T) {
	m := newModule(t)
	a1 := newItem(t, m, "a1")

	err := m.PatchItems([]string{a1.Id}, []string{"name"}, supplymanager.SupplyItem{Name: "Patched Name", Unit: "BAG"})
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}

	got, _ := m.GetItem(testTenant, a1.Id)
	if got.Name != "Patched Name" || got.Unit != "BOX" {
		t.Errorf("expected name Patched Name and unit BOX, got %v and %v", got.Name, got.Unit)
	}

	err = m.PatchItems([]string{a1.Id}, []string{"bogus"}, supplymanager.SupplyItem{Name: "Patched Name"})
	if model.KindOf(err) != model.ErrorInvalid { // replaced ErrUnknownField
		t.Errorf("expected ValidationError with ErrUnknownField, got %v", err)
	}
}

func TestDeactivateItem(t *testing.T) {
	m := newModule(t)
	a1 := newItem(t, m, "a1")
	l1 := newLocation(t, m, "Loc1")

	err := m.DeactivateItem(testTenant, a1.Id)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	items, err := m.ListItems(supplymanager.ListItemsArgs{TenantId: testTenant, ActiveOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	for _, item := range items {
		if item.Id == a1.Id {
			t.Errorf("expected deactivated item to not be listed")
		}
	}

	_, err = m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: a1.Id, LocationId: l1.Id, BatchNumber: "B1", Quantity: 10,
	})
	if model.KindOf(err) != model.ErrorInvalid {
		t.Errorf("expected ErrItemInactive, got %v", err)
	}
}

func TestDeleteItems(t *testing.T) {
	m := newModule(t)
	a1 := newItem(t, m, "empty")
	a2 := newItem(t, m, "with-batch")
	l1 := newLocation(t, m, "Loc1")

	_, err := m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: a2.Id, LocationId: l1.Id, BatchNumber: "B1", Quantity: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = m.DeleteItems([]string{a1.Id})
	if err != nil {
		t.Errorf("unexpected error deleting empty item: %v", err)
	}
	_, err = m.GetItem(testTenant, a1.Id)
	if model.KindOf(err) != model.ErrorNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}

	err = m.DeleteItems([]string{a2.Id})
	if model.KindOf(err) != model.ErrorInvalid {
		t.Errorf("expected ErrItemInUse, got %v", err)
	}
	_, err = m.GetItem(testTenant, a2.Id)
	if err != nil {
		t.Errorf("expected item to still be there, got error: %v", err)
	}

	err = m.DeleteItems([]string{"unknown"})
	if model.KindOf(err) != model.ErrorNotFound {
		t.Errorf("expected ErrItemNotFound, got %v", err)
	}
}

func TestListItems_PagedAndFiltered(t *testing.T) {
	m := newModule(t)
	// A, B, C created in order; B deactivated
	a := supplymanager.SupplyItem{TenantId: testTenant, Sku: "1", Name: "A", Unit: "BOX", IsActive: true}
	a, _ = m.CreateItem(a)
	b := supplymanager.SupplyItem{TenantId: testTenant, Sku: "2", Name: "B", Unit: "BOX", IsActive: true}
	b, _ = m.CreateItem(b)
	c := supplymanager.SupplyItem{TenantId: testTenant, Sku: "3", Name: "C", Unit: "BOX", IsActive: true}
	c, _ = m.CreateItem(c)

	_ = m.DeactivateItem(testTenant, b.Id)

	items, err := m.ListItems(supplymanager.ListItemsArgs{TenantId: testTenant, Limit: 2, Offset: 1})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(items) != 2 || items[0].Name != "B" || items[1].Name != "C" {
		t.Errorf("expected B, C, got %v", items)
	}

	itemsActive, err := m.ListItems(supplymanager.ListItemsArgs{TenantId: testTenant, ActiveOnly: true})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(itemsActive) != 2 || itemsActive[0].Name != "A" || itemsActive[1].Name != "C" {
		t.Errorf("expected A, C, got %v", itemsActive)
	}
}

func TestItemExists(t *testing.T) {
	m := newModule(t)
	active := newItem(t, m, "active")
	deact := newItem(t, m, "deact")
	_ = m.DeactivateItem(testTenant, deact.Id)

	exists, err := m.ItemExists(testTenant, active.Id)
	if !exists || err != nil {
		t.Errorf("expected true, nil, got %v, %v", exists, err)
	}

	exists, err = m.ItemExists(testTenant, deact.Id)
	if exists || err != nil {
		t.Errorf("expected false, nil, got %v, %v", exists, err)
	}

	exists, err = m.ItemExists(testTenant, "unknown")
	if exists || err != nil {
		t.Errorf("expected false, nil, got %v, %v", exists, err)
	}

	exists, err = m.ItemExists("tenant-2", active.Id)
	if exists || err != nil {
		t.Errorf("expected false, nil, got %v, %v", exists, err)
	}
}

func TestCreateItem_Rules(t *testing.T) {
	m := newModule(t)

	_, err := m.CreateItem(supplymanager.SupplyItem{TenantId: testTenant, Sku: "1", Name: "N", Unit: "BAG", IsActive: true})
	if model.KindOf(err) != model.ErrorInvalid {
		t.Errorf("expected ErrUnit, got %v", err)
	}

	_, err = m.CreateItem(supplymanager.SupplyItem{TenantId: testTenant, Sku: "1", Name: "", Unit: "BOX", IsActive: true})
	if model.KindOf(err) != model.ErrorInvalid {
		t.Errorf("expected ValidationError, got %v", err)
	}
}

func TestItemCard(t *testing.T) {
	m := newModule(t)
	item, _ := m.CreateItem(supplymanager.SupplyItem{TenantId: testTenant, Sku: "1", Name: "Surgical gloves", Unit: "PAIR", IsActive: true})

	name, unit, active, err := m.ItemCard(testTenant, item.Id)
	if err != nil || name != "Surgical gloves" || unit != "PAIR" || !active {
		t.Errorf("expected Surgical gloves, PAIR, true, nil, got %v, %v, %v, %v", name, unit, active, err)
	}

	_, _, _, err = m.ItemCard(testTenant, "unknown")
	if model.KindOf(err) != model.ErrorNotFound {
		t.Errorf("expected ErrNotFound, got %v", err)
	}
}

func TestCreateLocation_NameIsUniquePerTenant(t *testing.T) {
	m := newModule(t)
	_ = newLocation(t, m, "Store")

	_, err := m.CreateLocation(supplymanager.SupplyLocation{TenantId: testTenant, Name: "Store", IsActive: true})
	if model.KindOf(err) != model.ErrorConflict {
		t.Errorf("expected ErrLocationNameExists, got %v", err)
	}

	loc2, err2 := m.CreateLocation(supplymanager.SupplyLocation{TenantId: "tenant-2", Name: "Store", IsActive: true})
	if err2 != nil {
		t.Errorf("expected nil error, got %v", err2)
	}
	if loc2.Id == "" {
		t.Errorf("expected location to be created in tenant-2")
	}
}

func TestUpdateLocation(t *testing.T) {
	m := newModule(t)
	store := newLocation(t, m, "Store")
	_ = newLocation(t, m, "Counter")

	store.Name = "Counter"
	_, err := m.UpdateLocation(store)
	if model.KindOf(err) != model.ErrorConflict {
		t.Errorf("expected ErrLocationNameExists, got %v", err)
	}

	store.Name = "Warehouse"
	updated, err := m.UpdateLocation(store)
	if err != nil {
		t.Errorf("unexpected error: %v", err)
	}
	if updated.Name != "Warehouse" {
		t.Errorf("expected name to change to Warehouse")
	}
}

func TestDeleteLocations(t *testing.T) {
	m := newModule(t)
	empty := newLocation(t, m, "Empty")
	holding := newLocation(t, m, "Holding")
	item := newItem(t, m, "i1")

	_, err := m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: holding.Id, BatchNumber: "B1", Quantity: 10,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = m.DeleteLocations([]string{empty.Id})
	if err != nil {
		t.Errorf("unexpected error deleting empty location: %v", err)
	}

	err = m.DeleteLocations([]string{holding.Id})
	if model.KindOf(err) != model.ErrorInvalid {
		t.Errorf("expected ErrLocationInUse, got %v", err)
	}
}

func TestInactiveLocationRefusesReceipts(t *testing.T) {
	m := newModule(t)
	item := newItem(t, m, "i1")
	loc := newLocation(t, m, "Loc")

	loc.IsActive = false
	_, err := m.UpdateLocation(loc)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	_, err = m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: item.Id, LocationId: loc.Id, BatchNumber: "B1", Quantity: 10,
	})
	if model.KindOf(err) != model.ErrorInvalid {
		t.Errorf("expected ErrLocationInactive, got %v", err)
	}
}
func init() {
	// dummy use of newLocation just to avoid unused
}
