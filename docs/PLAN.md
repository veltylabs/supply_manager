---
PLAN: "feat: raise notify alerts — below minimum when an output crosses it, and the daily expiry review"
EXECUTOR: jules
REVIEWER: none
STATUS: running
SESSION: 1520402411300222046
---

> Read AGENTS.md at the repo root before any change.

# supply_manager: alerts below minimum and on expiring stock

## Why

Owner decision: *the stock modules raise an alert when an item crosses its minimum, and in a daily
review of expiring batches*. Alerts are `notify.Alert` values published with `notify.Raise` on the
module's `events.Publisher` (`notify` is a typed message **on** the `events` bus, not a second bus);
`alert_manager` stores them and shows them to the area. This module already knows both facts
(`ListLowStock`, `ListExpiring` in `reports.go`); it only never announced them.

| Rule | Behaviour |
|---|---|
| Below minimum | After `RegisterOutput` commits (this includes `UseInRoom`, which calls it), if the item's total over every location and batch went from above its `min_quantity` to at or below it → **one** alert for that item. Staying low does not repeat it. `TransferStock` never changes the total: no alert |
| Expiring | `ReviewExpiring(tenantId, withinDays)` (the application calls it once a day) → **one** summary alert when at least one batch with stock is expired or expires within `withinDays`; none when the list is empty |
| Switch | `Deps.AlertArea` is the application's `notify.Area` code. `""` or a nil `Publisher` disables the alerts (same convention as `MoneyScope` for cost facts) |
| The rule | `stock.Low(total, min)` and `stock.CrossedLow(before, after, min)` from `webtyp.com/stock` v0.1.4 — never re-written here |

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```

## Environment

```bash
export GOPRIVATE=webtyp.com/*,github.com/veltylabs/*
export GOFLAGS=-mod=mod
go get webtyp.com/stock@v0.1.4 webtyp.com/notify@v0.2.2 webtyp.com/msgtype@v0.1.0 webtyp.com/lang@v0.1.7
```

A PR that does not compile, or whose tests could not run, is a failed delivery. Never pause to ask
permission or report progress: when the Delivery check passes, commit, push and open the PR. Stuck
30 minutes: open the PR anyway listing what fails. Never edit `model_orm.go` by hand.

## APIs used

| Symbol | Exact signature | Tag |
|---|---|---|
| `stock.Low` | `func Low(total, minimum float64) bool` | `webtyp.com/stock` v0.1.4 |
| `stock.CrossedLow` | `func CrossedLow(before, after, minimum float64) bool` | v0.1.4 |
| `notify.Raise` | `func Raise(p events.Publisher, a Alert) error` (an invalid alert is not published, its error is returned) | `webtyp.com/notify` v0.2.2 |
| `notify.Alert` | `struct{ Code, Scope string; Level msgtype.Type; Subject, Body string; Source Ref }` | v0.2.2 |
| `notify.Ref` | `struct{ Type, ID string }` | v0.2.2 |
| `notify.Kind` | `struct{ Code string; Name, Help lang.Text; Area string }` | v0.2.2 |
| `notify.TopicAlert` | `"notify.alert"`; payload `*notify.Alert` | v0.2.2 |
| `msgtype.Warning` | `msgtype.Type` constant | `webtyp.com/msgtype` v0.1.0 |
| `time.NowUnix`, `time.FormatISO8601` | as already used in this repo | `webtyp.com/time` |

This repo: `ListStock(ListStockArgs{TenantId, ItemId}) ([]SupplyStockLevel, error)`,
`ListExpiring(ListExpiringArgs{TenantId, WithinDays}) ([]ExpiringStock, error)`,
`GetItem(tenantId, id string) (SupplyItem, error)`, `RegisterOutput(a RegisterOutputArgs)` (`module.go:387`).

If a signature differs, stop and name it in the PR; never adapt it by guessing.

## Stage 1 — `alerts.go` (new)

```go
package supplymanager

// Alert codes this module raises (notify.Alert.Code).
const (
	AlertBelowMinimum = "supply.below_minimum"
	AlertExpiring     = "supply.expiring"
)

// Source types of the alerts (notify.Ref.Type).
const (
	alertSourceItem     = "supply_item"
	alertSourceReview  = "supply_expiry_review"
)

const (
	textBelowMinimumName = "Supplies below minimum"
	textBelowMinimumHelp = "An item reached its minimum quantity after an output. Restock it."
	textExpiringName     = "Supplies expiring"
	textExpiringHelp     = "The daily review found batches with stock that are expired or expire soon."
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
```

`Deps` gains `AlertArea string // optional: the application's notify.Area code for this module's alerts; "" disables them`;
`Module` gains `alertArea string`, set in `New`.

## Stage 2 — below minimum in `RegisterOutput` (`module.go`)

1. At the start of `RegisterOutput`, after the argument checks and **before** the transaction:
   `before, err := m.totalItem(a.TenantId, a.ItemId)` (return the error).
2. After the commit and after the existing `publish`/`publishCosts` calls, and only when
   `len(out) > 0`: read the item with `GetItem(a.TenantId, a.ItemId)`, `after` with
   `totalItem`, and when `stock.CrossedLow(before, after, item.MinQuantity)`:

```go
alertErr := m.raise(notify.Alert{
	Code:    AlertBelowMinimum,
	Subject: item.Name,
	Body:    fmt.Sprintf(msgBelowMinimum, after, item.Unit, item.MinQuantity),
	Source:  notify.Ref{Type: alertSourceItem, ID: item.Id},
})
```
   with `const msgBelowMinimum = "%v %s left; minimum %v."` and `fmt` = `webtyp.com/fmt` (already
   imported in this package; never the stdlib `fmt`).
3. Errors after the commit (reading the item, the total, or `alertErr`) are returned **with** `out`,
   exactly like `publishCosts` does today (`return out, err`): the stock change is saved.
4. `ListLowStock` (`reports.go`) uses `stock.Low(total, p.MinQuantity)` instead of
   `p.MinQuantity <= 0` + `total <= p.MinQuantity`. Behaviour unchanged.

| Setup (min 10) | Call | Expected |
|---|---|---|
| total 12, alerts on | output 3 | one `notify.TopicAlert` event, code `supply.below_minimum`, scope = AlertArea, source `supply_item`/id, body `"9 <unit> left; minimum 10."` |
| total 12 | output 2 (lands on 10) | one alert |
| total 9 | output 1 | no alert |
| total 20 | output 5 | no alert |
| min 0 | any output | no alert |
| `AlertArea == ""` | output 3 from 12 | no alert, output ok |
| `Publisher == nil` | output 3 from 12 | no alert, output ok |
| transfer between two locations, total 12 → 12 | `TransferStock` | no alert |

## Stage 3 — the daily review (`reports.go`)

```go
// ReviewExpiring raises one alert when batches with stock are expired or expire within withinDays,
// and returns them. The application calls it once a day; this module has no scheduler.
func (m *Module) ReviewExpiring(tenantId string, withinDays int) ([]ExpiringStock, error)
```

1. `rows, err := m.ListExpiring(ListExpiringArgs{TenantId: tenantId, WithinDays: int64(withinDays)})`
   (use the real type of `WithinDays` in `ListExpiringArgs`; errors returned as they are).
2. `len(rows) == 0` → `rows, nil`, no alert.
3. Else count `expired` (rows with `Expired`) and `soon` (the rest) and
   `m.raise(notify.Alert{Code: AlertExpiring, Subject: textExpiringName, Body: fmt.Sprintf(msgExpiring, expired, soon, withinDays), Source: notify.Ref{Type: alertSourceReview, ID: time.FormatISO8601(time.NowUnix())[:10]}})`
   with `const msgExpiring = "%d expired batches and %d expiring within %d days."`; its error is returned with `rows`.

| Setup | Call | Expected |
|---|---|---|
| one batch expired, one expiring in 5 days, one in 60 | `ReviewExpiring(t, 30)` | 2 rows; one alert, body `"1 expired batches and 1 expiring within 30 days."` |
| nothing expiring | same | 0 rows, no alert |
| alerts off | first setup | 2 rows, no alert |
| `withinDays < 0` | — | the error `ListExpiring` returns today (`ErrDays`) |

## Stage 4 — translations

`langc sync .`; fill the `es` column for the new keys:

```json
"Supplies below minimum": ["Insumos bajo el mínimo"],
"Supplies expiring": ["Insumos por vencer"],
```

and the two `Help` texts.

## Stage 5 — tests (`tests/alerts_test.go`, new; same package and setup helpers as `tests/output_test.go`)

A publisher that records events (reuse the existing test publisher of `tests/events_test.go`).
One subtest per row of the Stage 2 and Stage 3 tables, plus:
`AlertKinds("supplies")` returns two kinds with those codes and that area. The alert is read from
the recorded event with `Topic == notify.TopicAlert` and `Payload.(*notify.Alert)`.

## Stage 6 — docs

`README.md`: section «Alerts» (the two rules, `AlertArea`, `AlertKinds`, that the application calls
`ReviewExpiring` daily). `docs/ARCHITECTURE.md`: one line.

## Delivery check

```bash
go build ./...
gotest
grep -n "total <= p.MinQuantity" reports.go   # → empty
git diff --stat main
```

## Stages

| # | Files | Done when |
|---|---|---|
| 1 | `alerts.go` (new), `module.go` (`Deps`, `New`) | builds |
| 2 | `module.go` (`RegisterOutput`), `reports.go` (`ListLowStock`) | Stage 2 table |
| 3 | `reports.go` (`ReviewExpiring`) | Stage 3 table |
| 4 | `lang.json` | `es` filled |
| 5 | `tests/alerts_test.go` | green |
| 6 | `README.md`, `docs/ARCHITECTURE.md` | docs match |
