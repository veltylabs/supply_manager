//go:build browser

package main

import (
	supplymanager "github.com/veltylabs/supply_manager"
	"github.com/veltylabs/supply_manager/seed"
	"github.com/veltylabs/supply_manager/ui"

	. "webtyp.com/dom"
	"webtyp.com/events/mock"
	. "webtyp.com/html"
	"webtyp.com/orm"
	"webtyp.com/router/loopback"
	"webtyp.com/storage/mem"
	"webtyp.com/unixid"
)

// demoTenantID is the only tenant of this in-browser demo.
const demoTenantID = "demo"

func main() {
	ids, err := unixid.NewUnixID()
	if err != nil {
		panic(err)
	}
	db := orm.New(mem.New())
	broker := &mock.Broker{}

	pm, err := supplymanager.New(db, supplymanager.Deps{IDs: ids, Publisher: broker})
	if err != nil {
		panic(err)
	}
	if _, err := seed.Load(pm, demoTenantID); err != nil {
		panic(err)
	}

	caller := loopback.WithTenant(demoTenantID, pm)
	screens := ui.Screens(caller)

	appNode := Div().Child(
		H1().Text(string(screens[0].Label)),
		P().Text("Demo"),
	)
	Append("body", appNode)
	select {}
}
