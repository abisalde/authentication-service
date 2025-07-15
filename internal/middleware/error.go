package middleware

import (
	"context"
	"errors"
	"log"

	"github.com/99designs/gqlgen/graphql"
	customErrors "github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
	"github.com/vektah/gqlparser/v2/gqlerror"
)

func ErrorPresenter(ctx context.Context, err error) *gqlerror.Error {
	var typedErr customErrors.TypedError
	if errors.As(err, &typedErr) {
		return &gqlerror.Error{
			Message: typedErr.Error(),
			Extensions: map[string]interface{}{
				"code": typedErr.ErrorType(),
			},
		}
	}
	var gqlErr *gqlerror.Error
	if errors.As(err, &gqlErr) && errors.Unwrap(gqlErr) == nil {
		return graphql.DefaultErrorPresenter(ctx, err)
	}

	log.Printf("Internal error: %+v", err)
	return &gqlerror.Error{
		Message: "Internal Server Error",
		Extensions: map[string]interface{}{
			"code": model.ErrorTypeInternalServerError,
		},
	}
}
