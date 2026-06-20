package mockgen

import "errors"

var (
	ErrNoMatchingExpectation = errors.New("mockgen: no matching expectation found")
	ErrCallCountMismatch     = errors.New("mockgen: call count mismatch")
	ErrInvalidInterface      = errors.New("mockgen: invalid interface type")
	ErrMethodNotFound        = errors.New("mockgen: method not found")
)
