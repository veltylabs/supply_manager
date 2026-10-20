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
	d := ddl.New(conn, ddlCompiler)
	if err := d.CreateTable(&supplymanager.SupplyItem{}); err != nil {
		return err
	}
	if err := d.CreateTable(&supplymanager.SupplyLocation{}); err != nil {
		return err
	}
	if err := d.CreateTable(&supplymanager.SupplyBatch{}); err != nil {
		return err
	}
	if err := d.CreateTable(&supplymanager.SupplyStockLevel{}); err != nil {
		return err
	}
	return d.CreateTable(&supplymanager.SupplyMovement{})
}
