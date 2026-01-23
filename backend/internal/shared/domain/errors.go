package domain

import "errors"

// Common errors.
var (
	ErrNotFound     = errors.New("not found")
	ErrInvalidInput = errors.New("invalid input")
	ErrForbidden    = errors.New("forbidden")
)
