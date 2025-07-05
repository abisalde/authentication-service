package middleware

import (
	"context"
	"log"

	"github.com/99designs/gqlgen/graphql"
)

func LoggingMiddleware(ctx context.Context, next graphql.Resolver) (res interface{}, err error) {
	rc := graphql.GetFieldContext(ctx)
	log.Printf("GraphQL: %s.%s called", rc.Object, rc.Field.Name)
	return next(ctx)
}
