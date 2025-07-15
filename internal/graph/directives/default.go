package directives

import (
	"context"
	"fmt"
	"log"
	"reflect"
	"strconv"

	"github.com/99designs/gqlgen/graphql"
)

type DefaultDirective struct{}

func NewDefaultDirective() *DefaultDirective {
	return &DefaultDirective{}
}

func (*DefaultDirective) Default(ctx context.Context, obj interface{}, next graphql.Resolver, value string) (interface{}, error) {
	result, err := next(ctx)
	if err != nil {
		return nil, err
	}

	if result != nil {

		return result, nil
	}

	field := graphql.GetFieldContext(ctx)
	if field == nil {
		return nil, fmt.Errorf("cannot apply @default, no field context")
	}

	log.Printf("@default invoked on field %s with default value %s", field.Field.Name, value)

	var finalValue interface{}

	if isZero(result) {
		fieldType := reflect.TypeOf(result)
		switch fieldType.Kind() {
		case reflect.String:
			finalValue = value
		case reflect.Bool:
			b, err := strconv.ParseBool(value)
			if err != nil {
				log.Printf("@default: failed to parse bool from %q: %v", value, err)
				finalValue = false
			} else {
				finalValue = b
			}
		case reflect.Int, reflect.Int32, reflect.Int64:
			i, err := strconv.ParseInt(value, 10, 64)
			if err != nil {
				log.Printf("@default: failed to parse int from %q: %v", value, err)
				finalValue = nil
			} else {

				finalValue = reflect.ValueOf(i).Convert(fieldType).Interface()
			}
		case reflect.Float32, reflect.Float64:
			f, err := strconv.ParseFloat(value, 64)
			if err != nil {
				log.Printf("@default: failed to parse float from %q: %v", value, err)
				finalValue = nil
			} else {
				finalValue = reflect.ValueOf(f).Convert(fieldType).Interface()
			}
		default:
			log.Printf("@default: unknown type %s, fallback to string", fieldType.String())
			finalValue = value
		}
	}

	return finalValue, nil

}

func isZero(x interface{}) bool {
	if x == nil {
		return true
	}
	v := reflect.ValueOf(x)
	switch v.Kind() {
	case reflect.Ptr, reflect.Interface, reflect.Slice, reflect.Map, reflect.Chan, reflect.Func:
		return v.IsNil()
	}

	return reflect.DeepEqual(x, reflect.Zero(v.Type()).Interface())
}
