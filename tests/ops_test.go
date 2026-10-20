package tests

import (
	"strings"
	"testing"

	"webtyp.com/time"

	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/router/loopback"
	"webtyp.com/stock"
)

// call runs one operation through the in-process caller, the way a transport would, and returns its error.
func call(t *testing.T, c router.Caller, op string, args model.Encodable, out model.Decodable) error {
	t.Helper()
	var got error
	c.Call(supplymanager.ModelName+"."+op, args, out, func(err error) { got = err })
	return got
}

func TestOperations_ThroughTheCaller(t *testing.T) {
	m := newModule(t)
	c := loopback.New(m)

	var loc supplymanager.SupplyLocation
	if err := call(t, c, supplymanager.OpUpsertLocation, &supplymanager.UpsertLocationArgs{Records: []supplymanager.SupplyLocation{
		{TenantId: testTenant, Name: "Store", IsActive: true},
	}}, &loc); err != nil || loc.Id == "" {
		t.Fatalf("upsert_location: %v, %+v", err, loc)
	}

	var prod supplymanager.SupplyItem
	if err := call(t, c, supplymanager.OpUpsertItem, &supplymanager.UpsertItemArgs{Records: []supplymanager.SupplyItem{
		{TenantId: testTenant, Sku: "p1", Name: "Surgical gloves", Unit: supplymanager.UnitBox, IsActive: true},
	}}, &prod); err != nil || prod.Id == "" {
		t.Fatalf("upsert_item: %v, %+v", err, prod)
	}

	var mv supplymanager.SupplyMovement
	recv := receive(prod.Id, loc.Id, "L1", time.NowUnix()+30*time.Day, 10)
	if err := call(t, c, supplymanager.OpReceiveStock, &recv, &mv); err != nil || mv.Quantity != 10 {
		t.Fatalf("receive_stock: %v, %+v", err, mv)
	}

	var levels supplymanager.SupplyStockLevelList
	if err := call(t, c, supplymanager.OpListStock, &supplymanager.ListStockArgs{TenantId: testTenant}, &levels); err != nil || len(levels) != 1 || levels[0].Quantity != 10 {
		t.Fatalf("list_stock: %v, %d rows", err, len(levels))
	}

	out := output(prod.Id, loc.Id, stock.InternalUse, 4)
	var sold supplymanager.SupplyMovementList
	if err := call(t, c, supplymanager.OpRegisterOutput, &out, &sold); err != nil || len(sold) != 1 || sold[0].Quantity != 4 {
		t.Fatalf("register_output: %v, %d movements", err, len(sold))
	}

	// the view's update: ids + the fields to change + a record with the values
	if err := call(t, c, supplymanager.OpUpdateItem, &supplymanager.UpdateItemArgs{
		Ids: []string{prod.Id}, Fields: []string{"name"}, Record: supplymanager.SupplyItem{Name: "Surgical gloves M"},
	}, nil); err != nil {
		t.Fatalf("update_item: %v", err)
	}
	if got, err := m.GetItem(testTenant, prod.Id); err != nil || got.Name != "Surgical gloves M" || got.Unit != supplymanager.UnitBox {
		t.Errorf("only the name must change: %+v, %v", got, err)
	}

	// the view's delete: ids only; an item with batches cannot go
	err := call(t, c, supplymanager.OpDeleteItem, &supplymanager.DeleteIdsArgs{Ids: []string{prod.Id}}, nil)
	if err == nil || !strings.Contains(err.Error(), string(supplymanager.ErrItemInUse)) {
		t.Errorf("deleting an item with batches: got %v, want ErrItemInUse", err)
	}
	if err := call(t, c, supplymanager.OpDeleteLocation, &supplymanager.DeleteIdsArgs{Ids: []string{loc.Id}}, nil); err == nil || !strings.Contains(err.Error(), string(supplymanager.ErrLocationInUse)) {
		t.Errorf("deleting a location with stock: got %v, want ErrLocationInUse", err)
	}
}
