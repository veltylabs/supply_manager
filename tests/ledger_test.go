//go:build !wasm

package tests


import (
	"testing"

	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/time"

	"webtyp.com/stock"
	"webtyp.com/stock/stocktest"
)

// idPair joins an id chosen by the conformance suite ("prod1", "loc1", "lotA") to the id the module generated.
// A slice scanned linearly, never a map: no module code or test uses map[K]V.
type idPair struct{ Suite, DB string }

// ledger adapts a Module to stocktest.Ledger so that the shared stock rules run against this module.
type ledger struct {
	t         *testing.T
	m         *supplymanager.Module
	items     []idPair
	locations []idPair
	lots      []idPair // Suite = lot id, DB = batch id
	lotOf     []idPair // Suite = lot id, DB = item id of that lot
}

func dbID(pairs []idPair, suite string) (string, bool) {
	for _, p := range pairs {
		if p.Suite == suite {
			return p.DB, true
		}
	}
	return "", false
}

func suiteID(pairs []idPair, db string) string {
	for _, p := range pairs {
		if p.DB == db {
			return p.Suite
		}
	}
	return db
}

func (l *ledger) item(suite string) string {
	if id, ok := dbID(l.items, suite); ok {
		return id
	}
	p := newItem(l.t, l.m, "sku-"+suite)
	l.items = append(l.items, idPair{Suite: suite, DB: p.Id})
	return p.Id
}

func (l *ledger) location(suite string) string {
	if id, ok := dbID(l.locations, suite); ok {
		return id
	}
	loc := newLocation(l.t, l.m, "loc "+suite)
	l.locations = append(l.locations, idPair{Suite: suite, DB: loc.Id})
	return loc.Id
}

func (l *ledger) Receive(item, location, lot string, expiry time.Unix, quantity float64) error {
	mv, err := l.m.ReceiveStock(supplymanager.ReceiveStockArgs{
		TenantId: testTenant, ItemId: l.item(item), LocationId: l.location(location),
		BatchNumber: lot, ExpirationDate: expiry, Quantity: quantity,
	})
	if err != nil {
		return err
	}
	if _, ok := dbID(l.lots, lot); !ok {
		l.lots = append(l.lots, idPair{Suite: lot, DB: mv.BatchId})
		l.lotOf = append(l.lotOf, idPair{Suite: lot, DB: l.item(item)})
	}
	return nil
}

// mapErr translates the module's errors to the suite's: only "not enough stock" differs.
func mapErr(err error) error {
	if err == supplymanager.ErrInsufficientStock {
		return stock.ErrInsufficient
	}
	return err
}

// Output maps the suite's stock.Sale to stock.InternalUse: supplies are not sold, so a consumption is their only output
// besides the write-off; the rule under test (first expiry first out) is the same.
func (l *ledger) Output(item, location string, m stock.Movement, quantity float64) ([]stock.Take, error) {
	if m == stock.Sale {
		m = stock.InternalUse
	}
	mvs, err := l.m.RegisterOutput(supplymanager.RegisterOutputArgs{
		TenantId: testTenant, ItemId: l.item(item), LocationId: l.location(location),
		Quantity: quantity, MovementType: string(m),
	})
	if err != nil {
		return nil, mapErr(err)
	}
	takes := make([]stock.Take, len(mvs))
	for i, mv := range mvs {
		takes[i] = stock.Take{LotID: suiteID(l.lots, mv.BatchId), Quantity: mv.Quantity}
	}
	return takes, nil
}

func (l *ledger) Transfer(item, from, to string, quantity float64) error {
	_, err := l.m.TransferStock(supplymanager.TransferStockArgs{
		TenantId: testTenant, ItemId: l.item(item), FromLocationId: l.location(from), ToLocationId: l.location(to), Quantity: quantity,
	})
	return mapErr(err)
}

func (l *ledger) Lots(item, location string) ([]stock.Lot, error) {
	levels, err := l.m.ListStock(supplymanager.ListStockArgs{TenantId: testTenant, ItemId: l.item(item), LocationId: l.location(location)})
	if err != nil {
		return nil, err
	}
	var out []stock.Lot
	for _, level := range levels {
		batch, err := l.m.GetBatch(testTenant, level.BatchId)
		if err != nil {
			return nil, err
		}
		out = append(out, stock.Lot{ID: suiteID(l.lots, level.BatchId), Expiry: batch.ExpirationDate, Quantity: level.Quantity})
	}
	return out, nil
}

func (l *ledger) Entries(item string) ([]stocktest.Entry, error) {
	mvs, err := l.m.ListMovements(supplymanager.ListMovementsArgs{TenantId: testTenant, ItemId: l.item(item), Limit: 1000})
	if err != nil {
		return nil, err
	}
	out := make([]stocktest.Entry, 0, len(mvs))
	for i := len(mvs) - 1; i >= 0; i-- { // the module lists newest first; the suite wants oldest first
		mv := mvs[i]
		out = append(out, stocktest.Entry{
			Type: stock.Movement(mv.MovementType), Location: suiteID(l.locations, mv.LocationId),
			LotID: suiteID(l.lots, mv.BatchId), Quantity: mv.Quantity,
		})
	}
	return out, nil
}

// TestStockConformance runs the shared stock rules (first expiry first out, expired lots unused, atomic failures,
// transfers, ledger balance, never negative) against this module.
func TestStockConformance(t *testing.T) {
	stocktest.Run(t, func(t *testing.T) stocktest.Ledger {
		return &ledger{t: t, m: newModule(t)}
	})
}
