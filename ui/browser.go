package ui

import (
	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/lang"
	"webtyp.com/router"
	"webtyp.com/view"
)

const (
	descSupply    lang.Text = "Stock levels, lot tracking, and storage location assignments"
	descItems     lang.Text = "Catalog of managed items, categories, and inventory configurations"
	descLocations lang.Text = "Storage facilities, rooms, and bin assignments"
)

// Screens declara las pantallas que este módulo ofrece al menú de la app.
func Screens(caller router.Caller) []view.Screen {
	return []view.Screen{{
		ID:    ID,
		Label: Label,
		Icon:  string(IconSupply),
		Help:  descSupply,
		Children: []view.Screen{
			{
				ID:         ID + ".items",
				Label:      "Artículos",
				Icon:       string(IconItems),
				Permission: "supply_item",
				Presenter:  supplymanager.NewItemView(caller, view.WithDescription(descItems)),
			},
			{
				ID:         ID + ".locations",
				Label:      "Ubicaciones",
				Icon:       string(IconLocations),
				Permission: "supply_location",
				Presenter:  supplymanager.NewLocationView(caller, view.WithDescription(descLocations)),
			},
		},
	}}
}
