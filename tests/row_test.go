package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
)

func TestItemsDeclaredOnFields(t *testing.T) {
	item := supplymanager.SupplyItem{
		Id:   "item1",
		Name: "Gloves",
		Sku:  "GLV-01",
	}
	itemRow := item.Item()
	if itemRow.Label != "Gloves" {
		t.Errorf("item.Label = %q, want %q", itemRow.Label, "Gloves")
	}
	if itemRow.Description != "GLV-01" {
		t.Errorf("item.Description = %q, want %q", itemRow.Description, "GLV-01")
	}

	loc := supplymanager.SupplyLocation{
		Id:   "loc1",
		Name: "Main Store",
	}
	locRow := loc.Item()
	if locRow.Label != "Main Store" {
		t.Errorf("loc.Label = %q, want %q", locRow.Label, "Main Store")
	}
	if locRow.Description != "" {
		t.Errorf("loc.Description = %q, want empty", locRow.Description)
	}
}
