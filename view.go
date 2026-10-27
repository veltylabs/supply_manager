package supplymanager

import (
	"webtyp.com/model"
	"webtyp.com/router"
	"webtyp.com/view"
)

const (
	titleItems     = "Artículos"
	titleLocations = "Ubicaciones"
)

// NewItemView construye el Presenter de los artículos: el motor que envuelve un renderizador (crudview u otro).
//
//	presenter := supplymanager.NewItemView(caller)
func NewItemView(caller router.Caller, opts ...view.Option) view.Presenter {
	b := view.NewCallerLister(view.NewRecords().Caller(caller),
		view.Ops{Module: ModelName, List: OpListItems, Save: OpUpsertItem, Update: OpUpdateItem, Delete: OpDeleteItem},
		func() model.ModelSlice { return &SupplyItemList{} })
	allOpts := append([]view.Option{view.WithTitle(titleItems)}, opts...)
	return view.New(b, &SupplyItem{Unit: UnitPiece, IsActive: true}, allOpts...)
}

// NewLocationView construye el Presenter de las ubicaciones.
//
//	presenter := supplymanager.NewLocationView(caller)
func NewLocationView(caller router.Caller, opts ...view.Option) view.Presenter {
	b := view.NewCallerLister(view.NewRecords().Caller(caller),
		view.Ops{Module: ModelName, List: OpListLocations, Save: OpUpsertLocation, Update: OpUpdateLocation, Delete: OpDeleteLocation},
		func() model.ModelSlice { return &SupplyLocationList{} })
	allOpts := append([]view.Option{view.WithTitle(titleLocations)}, opts...)
	return view.New(b, &SupplyLocation{IsActive: true}, allOpts...)
}
