package supplymanager
import "webtyp.com/model"

import (
	"webtyp.com/fmt"
	"webtyp.com/notify"
	"webtyp.com/time"

	"slices"

	"webtyp.com/stock"
)

// ListExpiring devuelve el stock (cantidad > 0) cuyo lote está vencido o vence dentro de WithinDays días, el más pronto primero.
// La regla es stock.Lot.ExpiresWithin; este módulo no la repite.
//
// Ejemplo:
//  exp, err := m.ListExpiring(ListExpiringArgs{TenantId: "t1", WithinDays: 30})
func (m *Module) ListExpiring(a ListExpiringArgs) ([]ExpiringStock, error) {
	if a.WithinDays < 0 {
		return nil, ErrDays
	}
	now := time.NowUnix()
	levels, err := m.ListStock(ListStockArgs{TenantId: a.TenantId})
	if err != nil {
		return nil, err
	}
	var out []ExpiringStock
	for _, level := range levels {
		batch, err := getBatch(m.db, a.TenantId, level.BatchId)
		if err != nil {
			return nil, err
		}
		lot := stock.Lot{ID: batch.Id, Expiry: batch.ExpirationDate, Quantity: level.Quantity}
		if !lot.ExpiresWithin(now, int(a.WithinDays)) {
			continue
		}
		item, err := getItem(m.db, a.TenantId, batch.ItemId)
		if err != nil {
			return nil, err
		}
		out = append(out, ExpiringStock{
			ItemId: item.Id, ItemName: item.Name, BatchId: batch.Id, BatchNumber: batch.BatchNumber,
			LocationId: level.LocationId, Quantity: level.Quantity, ExpirationDate: batch.ExpirationDate, Expired: lot.Expired(now),
		})
	}
	slices.SortStableFunc(out, func(x, y ExpiringStock) int {
		switch {
		case x.ExpirationDate < y.ExpirationDate:
			return -1
		case x.ExpirationDate > y.ExpirationDate:
			return 1
		}
		return 0
	})
	return out, nil
}

// ListLowStock devuelve los artículos activos con una cantidad mínima (> 0) cuyo total, en todas las ubicaciones y lotes, es
// igual o menor a esta, por nombre. Un módulo de alerta posterior lo llama; este módulo no emite ninguna alerta.
//
// Ejemplo:
//  low, err := m.ListLowStock(ListLowStockArgs{TenantId: "t1"})
func (m *Module) ListLowStock(a ListLowStockArgs) ([]LowStock, error) {
	items, err := m.ListItems(ListItemsArgs{TenantId: a.TenantId, ActiveOnly: true})
	if err != nil {
		return nil, err
	}
	var out []LowStock
	for _, p := range items {
		if p.MinQuantity <= 0 {
			continue
		}
		levels, err := m.ListStock(ListStockArgs{TenantId: a.TenantId, ItemId: p.Id})
		if err != nil {
			return nil, err
		}
		var total float64
		for _, level := range levels {
			total += level.Quantity
		}
		if stock.Low(total, p.MinQuantity) {
			out = append(out, LowStock{ItemId: p.Id, ItemName: p.Name, Total: total, MinQuantity: p.MinQuantity})
		}
	}
	return out, nil
}

// ReviewExpiring emite una alerta cuando hay lotes con stock vencidos o que vencen dentro de withinDays,
// y los devuelve. La aplicación lo llama una vez al día; este módulo no tiene planificador.
//
// Ejemplo:
//  exp, err := m.ReviewExpiring("t1", 30)
func (m *Module) ReviewExpiring(tenantId string, withinDays int) ([]ExpiringStock, error) {
	items, err := m.ListExpiring(ListExpiringArgs{TenantId: tenantId, WithinDays: int64(withinDays)})

	if len(items) == 0 {
		return items, err
	}

	var expired, soon int
	for _, item := range items {
		if item.Expired {
			expired++
		} else {
			soon++
		}
	}

	const msgExpiring = "%d expired batches and %d expiring within %d days."
	alertErr := m.raise(notify.Alert{
		Code:    AlertExpiring,
		Subject: string(textExpiringName),
		Body:    fmt.Sprintf(msgExpiring, expired, soon, withinDays),
		Source:  notify.Ref{Type: alertSourceReview, ID: time.FormatISO8601(time.NowUnix())[:10]},
	})

	if alertErr != nil {
		return items, alertErr
	}

	return items, err
}

// ItemExists indica si un artículo existe en el tenant y está activo. Otros módulos declaran el mismo método en una
// interfaz reducida propia; *Module la satisface sin importarlos.
//
// Ejemplo:
//  exists, err := m.ItemExists("t1", "1")
func (m *Module) ItemExists(tenantId, itemId string) (bool, error) {
	p, err := getItem(m.db, tenantId, itemId)
	if model.KindOf(err) == model.ErrorNotFound {
		return false, nil
	}
	if err != nil {
		return false, err
	}
	return p.IsActive, nil
}

// ItemCard devuelve lo que otro módulo captura sobre un artículo. Un artículo ausente o foráneo devuelve ErrNotFound.
//
// Ejemplo:
//  name, unit, active, err := m.ItemCard("t1", "1")
func (m *Module) ItemCard(tenantId, itemId string) (name, unit string, active bool, err error) {
	p, err := getItem(m.db, tenantId, itemId)
	if model.KindOf(err) == model.ErrorNotFound {
		return "", "", false, ErrNotFound
	}
	if err != nil {
		return "", "", false, err
	}
	return p.Name, p.Unit, p.IsActive, nil
}
