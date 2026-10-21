//go:build browser

package ui

import (
	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/fmt"
	"webtyp.com/layout/crudview"
	"webtyp.com/layout/platformd"
	"webtyp.com/model"
	"webtyp.com/router"
)

// Browser builds the screen of the module for the app's modules/browser.go: the group "Farmacia" with its two children,
// Artículos and Ubicaciones. There is no screen for stock, batches or movements in this version.
func Browser(caller router.Caller, ids model.IDGenerator, tenantID string) (platformd.UIModule, error) {
	itemsView, err := crudview.New(crudview.Config{
		ParentID:  ID + ".items",
		Presenter: supplymanager.NewItemView(caller),
		IDs:       ids,
		NewRecord: func() model.Model {
			return &supplymanager.SupplyItem{
				TenantId: tenantID,
				Unit:     supplymanager.UnitPiece,
				IsActive: true,
			}
		},
	})
	if err != nil {
		return nil, err
	}
	// The options of a closed list are set here, not in the model: ormc does not accept module constants inside
	// input.Select(...) in a model literal.
	itemsView.SetOptions("unit",
		fmt.KeyValue{Key: supplymanager.UnitPiece, Value: "Piece"},
		fmt.KeyValue{Key: supplymanager.UnitBox, Value: "Box"},
		fmt.KeyValue{Key: supplymanager.UnitPair, Value: "Pair"},
		fmt.KeyValue{Key: supplymanager.UnitMl, Value: "Milliliter"},
		fmt.KeyValue{Key: supplymanager.UnitGram, Value: "Gram"},
	)

	locationsView, err := crudview.New(crudview.Config{
		ParentID:  ID + ".locations",
		Presenter: supplymanager.NewLocationView(caller),
		IDs:       ids,
		NewRecord: func() model.Model {
			return &supplymanager.SupplyLocation{TenantId: tenantID, IsActive: true}
		},
	})
	if err != nil {
		return nil, err
	}

	return platformd.NewGroupUIModule(ID, Label, IconSupply, []platformd.UIModule{
		platformd.NewUIModule(ID+".items", "Artículos", IconItems, itemsView),
		platformd.NewUIModule(ID+".locations", "Ubicaciones", IconLocations, locationsView),
	}), nil
}
