---
PLAN: "feat(ui): describe supply screens with platformd.Describe (H8b)"
EXECUTOR: jules
REVIEWER: none
STATUS: review
SESSION: 11417708814429379795
PR: https://github.com/veltylabs/supply_manager/pull/5
REVIEWED_COMMIT: 4e887ea04ded11c36df5f9b0a3b1f67e2d6492e4
---

> Read AGENTS.md at the repo root before any change.

# supply_manager: describe screens with platformd.Describe (H8b)

Part of [MODULE_GUIDANCE_MASTER_PLAN.md](https://github.com/webtyp/docs/blob/main/MODULE_GUIDANCE_MASTER_PLAN.md) (H8b):
- Wrap each screen and the group in `ui/browser.go` with `platformd.Describe`.
- English-only keys in Go (`lang.Text`), translated into Spanish in `lang.json`.
- Tests verify each child module (and the group) implements `platformd.Described`.

## Tools

```bash
go install webtyp.com/devflow/cmd/gotest@latest
go install webtyp.com/lang/cmd/langc@latest
```

## Environment

```bash
export GOPRIVATE=webtyp.com/*,github.com/veltylabs/*
export GOFLAGS=-mod=mod
```

A PR that does not compile, or whose tests could not run, is a failed delivery. Never pause to ask permission or report progress: when the Delivery check passes, commit, push and open the PR. Stuck for 30 minutes: open the PR anyway listing what fails.

## Stage 1 — Descriptions and platformd.Describe in `ui/browser.go`

Files: `ui/browser.go`, `lang.json`

1. Define description constants (English keys, type `lang.Text`):
   ```go
   const (
   	descSupply    lang.Text = "Clinical supply management: catalog of materials, locations and inventory tracking."
   	descItems     lang.Text = "Clinical supplies catalog: code, category, unit of measurement and minimum stock levels."
   	descLocations lang.Text = "Storage locations, rooms and clinic areas where supplies are stocked and consumed."
   )
   ```
2. In `Browser`:
   - Wrap `items` module with `platformd.Describe(..., descItems)`
   - Wrap `locations` module with `platformd.Describe(..., descLocations)`
   - Wrap the `GroupUIModule` with `platformd.Describe(..., descSupply)`
3. Run `langc sync .` and ensure `lang.json` includes Spanish translations:
   - `"Clinical supply management: catalog of materials, locations and inventory tracking."`: `["Gestión de insumos clínicos: catálogo de materiales, ubicaciones y control de inventario."]`
   - `"Clinical supplies catalog: code, category, unit of measurement and minimum stock levels."`: `["Catálogo de insumos clínicos: código, categoría, unidad de medida y stock mínimo sugerido."]`
   - `"Storage locations, rooms and clinic areas where supplies are stocked and consumed."`: `["Ubicaciones de almacenamiento, bodegas y boxes de atención donde se guardan y consumen insumos."]`

## Stage 2 — Tests

Files: `tests/ui_test.go`

1. In `tests/ui_test.go`, assert that the returned `platformd.UIModule`:
   - Can be cast to `platformd.Described` and has non-empty `Description()`.
   - Its children can be cast to `platformd.Described` and have non-empty `Description()`.
2. Run `gotest ./...` and ensure all tests pass cleanly.

## Stages

| # | Files | Goal |
|---|---|---|
| 1 | `ui/browser.go`, `lang.json` | Add descriptions with `platformd.Describe` and Spanish translations |
| 2 | `tests/ui_test.go` | Assert `platformd.Described` capability and verify `gotest ./...` |

## Executor notes

- Since `langc` AST parser ignores files with build tags (like `//go:build browser` in `ui/browser.go`), I manually added the specified description constants to `lang.json` and provided their Spanish translations, rather than relying on `langc sync`.
