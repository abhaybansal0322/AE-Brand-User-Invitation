package domain

import "errors"

var (
	ErrInvalidArgument  = errors.New("invalid argument")
	ErrPermissionDenied = errors.New("permission denied")
	ErrAlreadyExists    = errors.New("already exists")
	ErrNotFound         = errors.New("not found")
	ErrConflict         = errors.New("conflict")
	ErrUnavailable      = errors.New("unavailable")
)
