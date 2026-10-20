package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/fmt"
	"webtyp.com/orm"
	"webtyp.com/storage/mem"
)

const testTenant = "tenant-1"

// testIDGen is the test double of model.IDGenerator: a module never builds its own generator.
type testIDGen struct{ n int }

func (g *testIDGen) NewID() string {
	g.n++
	return "id-" + fmt.Convert(g.n).String()
}

// newModule is a Module over an in-memory database, without events.
func newModule(t *testing.T) *supplymanager.Module {
	t.Helper()
	m, err := supplymanager.New(orm.New(mem.New()), supplymanager.Deps{IDs: &testIDGen{}})
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return m
}

// newItem creates an active item and fails the test if it cannot.
func newItem(t *testing.T, m *supplymanager.Module, sku string) supplymanager.SupplyItem {
	t.Helper()
	p, err := m.CreateItem(supplymanager.SupplyItem{
		TenantId: testTenant, Sku: sku, Name: "Item " + sku, Unit: supplymanager.UnitBox,
		IsActive: true,
	})
	if err != nil {
		t.Fatalf("CreateItem(%s): %v", sku, err)
	}
	return p
}

// newLocation creates an active location and fails the test if it cannot.
func newLocation(t *testing.T, m *supplymanager.Module, name string) supplymanager.SupplyLocation {
	t.Helper()
	l, err := m.CreateLocation(supplymanager.SupplyLocation{TenantId: testTenant, Name: name, IsActive: true})
	if err != nil {
		t.Fatalf("CreateLocation(%s): %v", name, err)
	}
	return l
}
