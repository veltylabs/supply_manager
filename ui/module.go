package ui

import "webtyp.com/svg"

// ID is the identity of the module's screen: the RBAC prefix on the server and the route of the navigation on the client.
const ID = "supply"

// Label is the text of the navigation entry.
const Label = "Insumos"

const (
	IconSupply    = svg.Icon("supply")
	IconItems     = svg.Icon("supply_items")
	IconLocations = svg.Icon("supply_locations")
)
