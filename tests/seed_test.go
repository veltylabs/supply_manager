package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
	"github.com/veltylabs/supply_manager/seed"
)

func TestSeed_LoadsValidatedDemoData(t *testing.T) {
	m := newModule(t)
	data, err := seed.Load(m, testTenant)
	if err != nil {
		t.Fatalf("seed.Load: %v", err)
	}
	if len(data.Locations) != 2 || len(data.Items) != 3 {
		t.Fatalf("2 locations and 3 items expected, got %d and %d", len(data.Locations), len(data.Items))
	}
	levels, err := m.ListStock(supplymanager.ListStockArgs{TenantId: testTenant})
	if err != nil || len(levels) != 3 {
		t.Fatalf("3 items received in the store plus 10 units moved to the counter make 4 levels, got %d, %v", len(levels), err)
	}
}
