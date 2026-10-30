package supplymanager

import (
	"webtyp.com/lang"
	"webtyp.com/msgtype"
	"webtyp.com/notify"
)

// Códigos de alerta que este módulo emite (notify.Alert.Code).
const (
	AlertBelowMinimum = "supply.below_minimum"
	AlertExpiring     = "supply.expiring"
)

// Tipos de origen de las alertas (notify.Ref.Type).
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

// AlertKinds es el catálogo de alertas que este módulo emite, para el notify.Registry de la aplicación.
// area es el código notify.Area de la aplicación (el mismo valor que Deps.AlertArea).
//
// Ejemplo:
//  registry.Register(supplymanager.AlertKinds("my_area"))
func AlertKinds(area string) []notify.Kind {
	return []notify.Kind{
		{Code: AlertBelowMinimum, Name: textBelowMinimumName, Help: textBelowMinimumHelp, Area: area},
		{Code: AlertExpiring, Name: textExpiringName, Help: textExpiringHelp, Area: area},
	}
}

// raise publica a cuando las alertas están activadas; si están desactivadas (sin Publisher o sin AlertArea) no hace nada.
func (m *Module) raise(a notify.Alert) error {
	if m.pub == nil || m.alertArea == "" {
		return nil
	}
	a.Scope, a.Level = m.alertArea, msgtype.Warning
	return notify.Raise(m.pub, a)
}

// totalItem es el stock del artículo en todas las ubicaciones y lotes.
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
