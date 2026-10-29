package supplymanager

import (
	"webtyp.com/model"
)

const (
	ErrNotFound         model.NotFound = "supply: not found"
	ErrItemNotFound     model.NotFound = "supply: item not found"
	ErrLocationNotFound model.NotFound = "supply: location not found"
	ErrBatchNotFound    model.NotFound = "supply: batch not found"

	ErrSkuExists           model.Conflict = "supply: an item with this SKU already exists"
	ErrRoomTaken           model.Conflict = "supply: another location already supplies this room"
	ErrLocationNameExists  model.Conflict = "supply: a location with this name already exists"
	ErrBatchExists         model.Conflict = "supply: this batch number already exists for the item"
	ErrConflict            model.Conflict = "supply: concurrent update, retry"
	ErrBatchExpiryMismatch model.Conflict = "supply: the batch exists with a different expiry date"
	ErrBatchCostMismatch   model.Conflict = "supply: the batch exists with a different unit cost"

	ErrNoLocationForRoom model.Invalid = "supply: no active location supplies this room"
	ErrInsufficientStock model.Invalid = "supply: not enough stock"
	ErrBatchExpired      model.Invalid = "supply: the batch is expired"
	ErrUnit              model.Invalid = "supply: unknown unit"
	ErrMovementType      model.Invalid = "supply: this movement type is not allowed here"
	ErrBatchNumber       model.Invalid = "supply: the batch number is required (NO-BATCH for a supply without lot)"
	ErrExpiry            model.Invalid = "supply: the expiry date cannot be negative"
	ErrUnitCost          model.Invalid = "supply: the unit cost cannot be negative"
	ErrItemInUse         model.Invalid = "supply: the item has batches"
	ErrLocationInUse     model.Invalid = "supply: the location has stock"
	ErrSameLocation      model.Invalid = "supply: the origin and the destination are the same location"
	ErrItemInactive      model.Invalid = "supply: the item is inactive"
	ErrLocationInactive  model.Invalid = "supply: the location is inactive"
	ErrNoRecords         model.Invalid = "supply: no records to save"
	ErrNoIds             model.Invalid = "supply: no ids given"
	ErrUnknownField      model.Invalid = "supply: unknown field in the update"
	ErrDays              model.Invalid = "supply: the days cannot be negative"
	ErrCannotRead        model.Invalid = "supply_manager: cannot read the request"
)
