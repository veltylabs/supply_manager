package supplymanager

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

const (
	titleItems     = "Items"
	titleLocations = "Locations"
)

// Item projects an item as a view.Item: the label of a row and its secondary text.
func (m *SupplyItem) Item() view.Item {
	return view.Item{ID: m.Id, Label: m.Name, Description: m.Sku}
}

// Item projects a location as a view.Item.
func (m *SupplyLocation) Item() view.Item {
	return view.Item{ID: m.Id, Label: m.Name}
}

// NewItemView builds the Presenter of the items: the engine a renderer (crudview or any other) wraps. The app picks
// the renderer; this module imports only view, model and router.
func NewItemView(caller router.Caller) view.Presenter {
	b := view.NewCallerLister(caller,
		view.Ops{Module: ModelName, List: OpListItems, Save: OpUpsertItem, Update: OpUpdateItem, Delete: OpDeleteItem},
		func() model.ModelSlice { return &SupplyItemList{} })
	return view.New(b, &SupplyItem{}, view.WithTitle(titleItems))
}

// NewLocationView builds the Presenter of the locations.
func NewLocationView(caller router.Caller) view.Presenter {
	b := view.NewCallerLister(caller,
		view.Ops{Module: ModelName, List: OpListLocations, Save: OpUpsertLocation, Update: OpUpdateLocation, Delete: OpDeleteLocation},
		func() model.ModelSlice { return &SupplyLocationList{} })
	return view.New(b, &SupplyLocation{}, view.WithTitle(titleLocations))
}
