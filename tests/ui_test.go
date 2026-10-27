//go:build browser

package tests

import (
	"testing"

	"github.com/veltylabs/supply_manager/ui"
	"webtyp.com/model"
	"webtyp.com/router"
)

// mockCaller is a router.Caller that records the last call.
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

var _ router.Caller = (*mockCaller)(nil)

func TestUI_Screens(t *testing.T) {
	caller := &mockCaller{}
	screens := ui.Screens(caller)
	if len(screens) != 1 {
		t.Fatalf("len(screens) = %d, want 1", len(screens))
	}
	grp := screens[0]
	if err := grp.Validate(); err != nil {
		t.Fatalf("grp.Validate() error: %v", err)
	}
	if grp.ID != ui.ID {
		t.Errorf("grp.ID = %q, want %q", grp.ID, ui.ID)
	}
	if len(grp.Children) != 2 {
		t.Fatalf("len(grp.Children) = %d, want 2", len(grp.Children))
	}
	for _, child := range grp.Children {
		if err := child.Validate(); err != nil {
			t.Errorf("child %s Validate() error: %v", child.ID, err)
		}
		if child.Presenter.Description() == "" {
			t.Errorf("child %s Description() is empty", child.ID)
		}
	}
}
