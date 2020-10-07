package resources

import (
	"errors"
)

var (
	// ErrInvalidPvc may be returned if pvc values are not valid
	ErrInvalidPvc = errors.New("exactly one of pvc name or pvc spec must be set")
)
