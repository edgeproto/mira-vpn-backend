package repositories

import "errors"

// ErrNotFound is returned when a requested record does not exist.
var ErrNotFound = errors.New("not found")
