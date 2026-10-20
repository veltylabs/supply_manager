// Package seed loads demo data through the module's own methods, so every row is validated like a real one.
package seed

import (
	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/fmt"
	"webtyp.com/time"
)

// Data is what Load created, for the demos that build on it.
type Data struct {
	Locations []supplymanager.SupplyLocation
	Items     []supplymanager.SupplyItem
}
const (


	locStore = "Bodega"
	locRoom  = "Box 1"

	expiryDays   = 365 // the syringes expire within a year
	syringeCost  = 120 // whole pesos per unit
	demoQuantity = 100
)

// Load creates two locations, three items and one batch of each received in the store: the gloves without lot, the
// syringes with a lot, an expiry date and a cost, the gauze with a lot and no expiry.
func Load(m *supplymanager.Module, tenantID string) (Data, error) {
	data := Data{}
	for _, name := range []string{locStore, locRoom} {
		created, err := m.CreateLocation(supplymanager.SupplyLocation{TenantId: tenantID, Name: name, IsActive: true})
		if err != nil {
			return data, fmt.Err("seed: CreateLocation", name, err)
		}
		data.Locations = append(data.Locations, created)
	}

	items := []supplymanager.SupplyItem{
		{TenantId: tenantID, Sku: "GUANTES", Name: "Guantes de examen", Unit: supplymanager.UnitPair, MinQuantity: 50, IsActive: true},
		{TenantId: tenantID, Sku: "JERINGA", Name: "Jeringa 5 ml", Unit: supplymanager.UnitPiece, MinQuantity: 20, IsActive: true},
		{TenantId: tenantID, Sku: "GASA", Name: "Gasa estéril", Unit: supplymanager.UnitBox, IsActive: true},
	}
	for _, it := range items {
		created, err := m.CreateItem(it)
		if err != nil {
			return data, fmt.Err("seed: CreateItem", it.Sku, err)
		}
		data.Items = append(data.Items, created)
	}

	receipts := []struct {
		batch  string
		expiry int64
		cost   int64
	}{
		{supplymanager.NoBatchNumber, 0, 0},
		{"LOT-JER-1", int64(time.NowUnix() + time.Unix(expiryDays)*time.Day), syringeCost},
		{"LOT-GASA-1", 0, 0},
	}
	for i, it := range data.Items {
		if _, err := m.ReceiveStock(supplymanager.ReceiveStockArgs{
			TenantId: tenantID, ItemId: it.Id, LocationId: data.Locations[0].Id, BatchNumber: receipts[i].batch,
			ExpirationDate: time.Unix(receipts[i].expiry), UnitCost: receipts[i].cost, Quantity: demoQuantity,
		}); err != nil {
			return data, fmt.Err("seed: ReceiveStock", it.Sku, err)
		}
	}
	return data, nil
}
