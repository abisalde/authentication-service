package directives

import (
	"context"
	"fmt"
	"regexp"
	"strings"

	"github.com/99designs/gqlgen/graphql"
	"github.com/abisalde/authentication-service/internal/utils/validator"

	customErrors "github.com/abisalde/authentication-service/internal/graph/errors"
	"github.com/abisalde/authentication-service/internal/graph/model"
)

type ConstraintInput struct {
}

func NewConstraint() *ConstraintInput {
	return &ConstraintInput{}
}

func (c *ConstraintInput) Constraints(ctx context.Context, obj interface{}, next graphql.Resolver, format *string, minLength *int32, maxLength *int32, pattern *string, min *float64, max *float64) (interface{}, error) {
	value, err := next(ctx)
	if err != nil {
		return nil, err
	}

	switch v := value.(type) {
	case string:
		if minLength != nil && len(v) < int(*minLength) {
			return nil, customErrors.NewTypedError(
				fmt.Sprintf("Minimum length is %d", *minLength),
				model.ErrorTypeBadRequest,
				map[string]interface{}{"minLength": *minLength, "value": len(v)})

		}
		if maxLength != nil && len(v) > int(*maxLength) {
			return nil, customErrors.NewTypedError(
				fmt.Sprintf("Maximum length is %d", *maxLength),
				model.ErrorTypeBadRequest,
				map[string]interface{}{"maxLength": *maxLength, "value": len(v)})

		}
		if format != nil {
			switch strings.ToLower(*format) {
			case "email":
				err := validator.ValidateEmail(v)
				if err != nil {
					return nil, err
				}
			case "url":
				urlRegex := regexp.MustCompile(`^https?://`)
				if !urlRegex.MatchString(v) {
					return nil, customErrors.NewTypedError(
						"Invalid Url Format",
						model.ErrorTypeBadRequest,
						map[string]interface{}{"url": v},
					)

				}
			case "password":
				if err := validator.ValidatePassword(v); err != nil {
					return nil, err
				}
			}
		}
		if pattern != nil {
			reg := regexp.MustCompile(*pattern)
			if !reg.MatchString(v) {
				return nil, customErrors.NewTypedError(
					fmt.Sprintf("Value does not match pattern %s", *pattern),
					model.ErrorTypeBadRequest,
					map[string]interface{}{"constraint": "pattern", "pattern": *pattern},
				)
			}
		}
	case float64:
		if min != nil && v < *min {
			return nil, customErrors.NewTypedError(
				fmt.Sprintf("Minimum value is %f", *min),
				model.ErrorTypeBadRequest,
				map[string]interface{}{"constraint": "min", "value": v},
			)
		}
		if max != nil && v > *max {
			return nil, customErrors.NewTypedError(
				fmt.Sprintf("Maximum Value is %f", *max),
				model.ErrorTypeBadRequest,
				map[string]interface{}{"constraint": "max", "value": v},
			)
		}
	}

	return value, nil
}
