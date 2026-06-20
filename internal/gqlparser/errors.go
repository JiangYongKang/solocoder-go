package gqlparser

import (
	"errors"
	"fmt"
)

var (
	ErrTypeNotFound         = errors.New("gqlparser: type not found")
	ErrFieldNotFound        = errors.New("gqlparser: field not found")
	ErrTypeAlreadyExists    = errors.New("gqlparser: type already exists")
	ErrInvalidSDL           = errors.New("gqlparser: invalid SDL syntax")
	ErrInvalidQuery         = errors.New("gqlparser: invalid query syntax")
	ErrResolverNotFound     = errors.New("gqlparser: resolver not found")
	ErrNestedTooDeep        = errors.New("gqlparser: query nested too deep")
	ErrMissingRequiredField = errors.New("gqlparser: missing required field")
	ErrInvalidArgType       = errors.New("gqlparser: invalid argument type")
	ErrInvalidArgValue      = errors.New("gqlparser: invalid argument value")
	ErrUnknownOperation     = errors.New("gqlparser: unknown operation type")
	ErrDataLoaderNotReady   = errors.New("gqlparser: dataloader not ready")
	ErrDataLoaderCleared    = errors.New("gqlparser: dataloader request cleared")
)

func NewValidationError(path, format string, args ...interface{}) *ValidationError {
	return &ValidationError{
		Path:    path,
		Message: fmt.Sprintf(format, args...),
	}
}
