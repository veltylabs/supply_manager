package supplymanager

import (
	"webtyp.com/orm"
	"webtyp.com/router"
)

// domainError is a rule of the module that was not met; every message starts with "supply: ".
type domainError string

func (e domainError) Error() string { return string(e) }

const (
	// 404
	ErrNotFound         domainError = "supply: not found"
	ErrItemNotFound     domainError = "supply: item not found"
	ErrLocationNotFound domainError = "supply: location not found"
	ErrBatchNotFound    domainError = "supply: batch not found"

	// 409
	ErrSkuExists           domainError = "supply: an item with this SKU already exists"
	ErrRoomTaken           domainError = "supply: another location already supplies this room"
	ErrLocationNameExists  domainError = "supply: a location with this name already exists"
	ErrBatchExists         domainError = "supply: this batch number already exists for the item"
	ErrConflict            domainError = "supply: concurrent update, retry"
	ErrBatchExpiryMismatch domainError = "supply: the batch exists with a different expiry date"
	ErrBatchCostMismatch   domainError = "supply: the batch exists with a different unit cost"

	// 400
	ErrNoLocationForRoom domainError = "supply: no active location supplies this room"
	ErrInsufficientStock domainError = "supply: not enough stock"
	ErrBatchExpired      domainError = "supply: the batch is expired"
	ErrUnit              domainError = "supply: unknown unit"
	ErrMovementType      domainError = "supply: this movement type is not allowed here"
	ErrBatchNumber       domainError = "supply: the batch number is required (NO-BATCH for a supply without lot)"
	ErrExpiry            domainError = "supply: the expiry date cannot be negative"
	ErrUnitCost          domainError = "supply: the unit cost cannot be negative"
	ErrItemInUse         domainError = "supply: the item has batches"
	ErrLocationInUse     domainError = "supply: the location has stock"
	ErrSameLocation      domainError = "supply: the origin and the destination are the same location"
	ErrItemInactive      domainError = "supply: the item is inactive"
	ErrLocationInactive  domainError = "supply: the location is inactive"
	ErrNoRecords         domainError = "supply: no records to save"
	ErrNoIds             domainError = "supply: no ids given"
	ErrUnknownField      domainError = "supply: unknown field in the update"
	ErrDays              domainError = "supply: the days cannot be negative"
)

// ValidationError wraps a model validation failure: 400.
type ValidationError struct{ Err error }

func (v ValidationError) Error() string { return v.Err.Error() }

// decodeError is a request body that could not be read: always 400.
type decodeError struct{ err error }

const msgDecode = "supply: cannot read the request: "

func (d decodeError) Error() string { return msgDecode + d.err.Error() }

// writeError answers err with its status and, always, its text. 400 = invalid input · 404 = not found ·
// 409 = conflict · 500 = anything else.
func writeError(ctx router.Context, err error) {
	ctx.WriteStatus(statusOf(err))
	ctx.Write([]byte(err.Error()))
}

func statusOf(err error) int {
	switch e := err.(type) {
	case decodeError, ValidationError:
		return 400
	case domainError:
		switch e {
		case ErrNotFound, ErrItemNotFound, ErrLocationNotFound, ErrBatchNotFound:
			return 404
		case ErrSkuExists, ErrLocationNameExists, ErrRoomTaken, ErrBatchExists, ErrConflict, ErrBatchExpiryMismatch, ErrBatchCostMismatch:
			return 409
		default:
			return 400
		}
	}
	if orm.IsNotFound(err) {
		return 404
	}
	return 500
}
