package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/router"
	"webtyp.com/router/mock"
)

// TestMount_EveryOperationDeclaresItsResourceAndAction: an operation without Requires would be unreachable (zero Access is
// guarded) and must fail the build of the app, so the module must mount all of them with a resource and an action.
func TestMount_EveryOperationDeclaresItsResourceAndAction(t *testing.T) {
	var _ router.OperationModule = newModule(t) // the module is an OperationModule

	r := &mock.Router{}
	newModule(t).MountOperations(r)

	wantOps := []string{
		supplymanager.OpListItems, supplymanager.OpGetItem, supplymanager.OpCreateItem, supplymanager.OpUpdateItem,
		supplymanager.OpUpsertItem, supplymanager.OpDeactivateItem, supplymanager.OpDeleteItem,
		supplymanager.OpListLocations, supplymanager.OpUpsertLocation, supplymanager.OpUpdateLocation, supplymanager.OpDeleteLocation,
		supplymanager.OpListBatches, supplymanager.OpReceiveStock, supplymanager.OpRegisterEntry, supplymanager.OpRegisterOutput,
		supplymanager.OpTransferStock, supplymanager.OpListStock, supplymanager.OpListMovements, supplymanager.OpListExpiring,
		supplymanager.OpListLowStock,
	}
	routes := r.Routes()
	if len(routes) != len(wantOps) {
		t.Fatalf("%d operations mounted, want %d", len(routes), len(wantOps))
	}
	for _, info := range routes {
		if info.Resource == "" || info.Action == 0 {
			t.Errorf("%s %s must declare a resource and an action, got %q / %v", info.Method, info.Path, info.Resource, info.Action)
		}
	}
}
