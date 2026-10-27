package tests

import (
	"testing"
	"webtyp.com/model"

	supplymanager "github.com/veltylabs/supply_manager"
)

func TestRowsDeclaredOnFields(t *testing.T) {
	item := supplymanager.SupplyItem{
		Id:   "item1",
		Name: "Gloves",
		Sku:  "GLV-01",
	}
	itemRow := model.RowOf(&item)
	if itemRow.Title != "Gloves" {
		t.Errorf("item.Title = %q, want %q", itemRow.Title, "Gloves")
	}
	if itemRow.Detail != "GLV-01" {
		t.Errorf("item.Detail = %q, want %q", itemRow.Detail, "GLV-01")
	}

	loc := supplymanager.SupplyLocation{
		Id:   "loc1",
		Name: "Main Store",
	}
	locRow := model.RowOf(&loc)
	if locRow.Title != "Main Store" {
		t.Errorf("loc.Title = %q, want %q", locRow.Title, "Main Store")
	}
	if locRow.Detail != "" {
		t.Errorf("loc.Detail = %q, want empty", locRow.Detail)
	}
}
