package handlers

import "errors"

var (
	errNoRowsProvided = errors.New("no rows provided")
	errRowNotFound    = errors.New("row not found")

	errNoGeometryProvided = errors.New("no geometry provided")
	errNoPkeyProvided     = errors.New("an update is missing its pkey")
)
