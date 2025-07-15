package errors

import (
	"fmt"

	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

type TypedError interface {
	error
	ErrorType() model.ErrorType
}

type typedError struct {
	err       error
	errorType model.ErrorType
}

func (e *typedError) Error() string              { return e.err.Error() }
func (e *typedError) ErrorType() model.ErrorType { return e.errorType }

func (e *typedError) Unwrap() error {
	return e.err
}

func NewTypedError(message string, code model.ErrorType, extraExtensions map[string]interface{}) *gqlerror.Error {
	extensions := map[string]interface{}{
		"code": code,
	}

	for k, v := range extraExtensions {
		extensions[k] = v
	}

	return &gqlerror.Error{
		Message:    message,
		Extensions: extensions,
	}
}

func InternalServerError(message string, args ...any) error {
	return &typedError{err: fmt.Errorf(message, args...), errorType: model.ErrorTypeInternalServerError}
}
