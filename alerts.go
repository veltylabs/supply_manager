package supplymanager

import (
	"webtyp.com/lang"
	"webtyp.com/msgtype"
	"webtyp.com/notify"
)

// Alert codes this module raises (notify.Alert.Code).
const (
	AlertBelowMinimum = "supply.below_minimum"
	AlertExpiring     = "supply.expiring"
)

// Source types of the alerts (notify.Ref.Type).
const (
	alertSourceItem   = "supply_item"
	alertSourceReview = "supply_expiry_review"
)

const (
	textBelowMinimumName lang.Text = "Supplies below minimum"
	textBelowMinimumHelp lang.Text = "An item reached its minimum quantity after an output. Restock it."
	textExpiringName     lang.Text = "Supplies expiring"
	textExpiringHelp     lang.Text = "The daily review found batches with stock that are expired or expire soon."
)

// AlertKinds is the catalog of alerts this module raises, for the application's notify.Registry.
// area is the application's notify.Area code (the same value as Deps.AlertArea).
func AlertKinds(area string) []notify.Kind {
	return []notify.Kind{
		{Code: AlertBelowMinimum, Name: textBelowMinimumName, Help: textBelowMinimumHelp, Area: area},
		{Code: AlertExpiring, Name: textExpiringName, Help: textExpiringHelp, Area: area},
	}
}

// raise publishes a when alerts are on; off (no Publisher or no AlertArea) it does nothing.
func (m *Module) raise(a notify.Alert) error {
	if m.pub == nil || m.alertArea == "" {
		return nil
	}
	a.Scope, a.Level = m.alertArea, msgtype.Warning
	return notify.Raise(m.pub, a)
}

// totalItem is the item's stock over every location and batch.
func (m *Module) totalItem(tenantId, id string) (float64, error) {
	levels, err := m.ListStock(ListStockArgs{TenantId: tenantId, ItemId: id})
	if err != nil {
		return 0, err
	}
	var total float64
	for _, l := range levels {
		total += l.Quantity
	}
	return total, nil
}
