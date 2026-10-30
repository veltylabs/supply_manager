// Package migrate reconciles the schema this module owns. It is a package of its own, never called by New, so that nothing
// on a WASM build's import path pulls webtyp.com/ddl: schema work is deploy-time work.
package migrate

import (
	supplymanager "github.com/veltylabs/supply_manager"
	"webtyp.com/ddl"
)

// Migrate creates the five tables in foreign-key order: item and location first, then the batch that references the
// item, then the stock level and the movement that reference location and batch.
func Migrate(conn ddl.Execer, ddlCompiler ddl.Compiler) error {
	// Sync y no CreateTable: crea la tabla que falta y además agrega las columnas que un modelo
	// ganó, así un campo nuevo llega a una base ya instalada. Flujo: https://github.com/webtyp/ddl/blob/main/docs/diagrams/SYNC_FLOW.md
	return ddl.New(conn, ddlCompiler).Sync(
		&supplymanager.SupplyItem{},
		&supplymanager.SupplyLocation{},
		&supplymanager.SupplyBatch{},
		&supplymanager.SupplyStockLevel{},
		&supplymanager.SupplyMovement{},
	)
}
