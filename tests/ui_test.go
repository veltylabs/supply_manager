//go:build browser

package tests

import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
	"github.com/veltylabs/supply_manager/ui"
	"webtyp.com/layout/crudview"
	"webtyp.com/layout/platformd"
	"webtyp.com/model"
)

// mockCaller is a router.Caller that records the last call and answers it with onCall.
type mockCaller struct {
	lastOp string
	onCall func(op string, args model.Encodable, into model.Decodable) error
}

func (m *mockCaller) Call(op string, args model.Encodable, into model.Decodable, done func(err error)) {
	m.lastOp = op
	if m.onCall != nil {
		done(m.onCall(op, args, into))
		return
	}
	done(nil)
}

func (m *mockCaller) Dispatch(op string, args model.Encodable) { m.lastOp = op }

func TestBrowser_IsAGroupOfItemsAndLocations(t *testing.T) {
	caller := &mockCaller{}
	screen, err := ui.Browser(caller, &testIDGen{}, testTenant)
	if err != nil {
		t.Fatalf("ui.Browser: %v", err)
	}
	if screen.ModelName() != ui.ID || screen.Label() != ui.Label {
		t.Errorf("the group must be %q / %q, got %q / %q", ui.ID, ui.Label, screen.ModelName(), screen.Label())
	}
	group, ok := screen.(platformd.GroupModule)
	if !ok {
		t.Fatalf("expected a platformd.GroupModule, got %T", screen)
	}
	children := group.Children()
	if len(children) != 2 || children[0].Label() != "Artículos" || children[1].Label() != "Ubicaciones" {
		t.Fatalf("two children Artículos and Ubicaciones expected, got %d", len(children))
	}
	if desc, ok := screen.(platformd.Described); !ok || desc.Description() == "" {
		t.Errorf("screen must be a platformd.Described with a description")
	}
	for i, c := range children {
		if _, ok := c.View().(*crudview.CrudView); !ok {
			t.Errorf("child %d must be a *crudview.CrudView, got %T", i, c.View())
		}
		if desc, ok := c.(platformd.Described); !ok || desc.Description() == "" {
			t.Errorf("child %d must be a platformd.Described with a description", i)
		}
	}
}

func TestBrowser_ActivatingLoadsTheItems(t *testing.T) {
	caller := &mockCaller{}
	screen, err := ui.Browser(caller, &testIDGen{}, testTenant)
	if err != nil {
		t.Fatal(err)
	}
	first := screen.(platformd.GroupModule).Children()[0]
	first.View().(*crudview.CrudView).Init(nil)
	screen.Activate()

	if want := supplymanager.ModelName + "." + supplymanager.OpListItems; caller.lastOp != want {
		t.Errorf("activating the group must list the items: last op %q, want %q", caller.lastOp, want)
	}
}
