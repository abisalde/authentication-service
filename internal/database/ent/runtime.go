// Code generated by ent, DO NOT EDIT.

package ent

import (
	"time"

	"github.com/abisalde/authentication-service/internal/database/ent/schema"
	"github.com/abisalde/authentication-service/internal/database/ent/user"
)

// The init function reads all schema descriptors with runtime code
// (default values, validators, hooks and policies) and stitches it
// to their package variables.
func init() {
	userMixin := schema.User{}.Mixin()
	userMixinFields0 := userMixin[0].Fields()
	_ = userMixinFields0
	userMixinFields1 := userMixin[1].Fields()
	_ = userMixinFields1
	userFields := schema.User{}.Fields()
	_ = userFields
	// userDescCreatedAt is the schema descriptor for created_at field.
	userDescCreatedAt := userMixinFields0[0].Descriptor()
	// user.DefaultCreatedAt holds the default value on creation for the created_at field.
	user.DefaultCreatedAt = userDescCreatedAt.Default.(func() time.Time)
	// userDescUpdatedAt is the schema descriptor for updated_at field.
	userDescUpdatedAt := userMixinFields0[1].Descriptor()
	// user.DefaultUpdatedAt holds the default value on creation for the updated_at field.
	user.DefaultUpdatedAt = userDescUpdatedAt.Default.(func() time.Time)
	// user.UpdateDefaultUpdatedAt holds the default value on update for the updated_at field.
	user.UpdateDefaultUpdatedAt = userDescUpdatedAt.UpdateDefault.(func() time.Time)
	// userDescStreetName is the schema descriptor for street_name field.
	userDescStreetName := userMixinFields1[0].Descriptor()
	// user.DefaultStreetName holds the default value on creation for the street_name field.
	user.DefaultStreetName = userDescStreetName.Default.(string)
	// user.StreetNameValidator is a validator for the "street_name" field. It is called by the builders before save.
	user.StreetNameValidator = userDescStreetName.Validators[0].(func(string) error)
	// userDescCity is the schema descriptor for city field.
	userDescCity := userMixinFields1[1].Descriptor()
	// user.DefaultCity holds the default value on creation for the city field.
	user.DefaultCity = userDescCity.Default.(string)
	// user.CityValidator is a validator for the "city" field. It is called by the builders before save.
	user.CityValidator = userDescCity.Validators[0].(func(string) error)
	// userDescZipCode is the schema descriptor for zip_code field.
	userDescZipCode := userMixinFields1[2].Descriptor()
	// user.DefaultZipCode holds the default value on creation for the zip_code field.
	user.DefaultZipCode = userDescZipCode.Default.(string)
	// user.ZipCodeValidator is a validator for the "zip_code" field. It is called by the builders before save.
	user.ZipCodeValidator = userDescZipCode.Validators[0].(func(string) error)
	// userDescCountry is the schema descriptor for country field.
	userDescCountry := userMixinFields1[3].Descriptor()
	// user.DefaultCountry holds the default value on creation for the country field.
	user.DefaultCountry = userDescCountry.Default.(string)
	// user.CountryValidator is a validator for the "country" field. It is called by the builders before save.
	user.CountryValidator = userDescCountry.Validators[0].(func(string) error)
	// userDescState is the schema descriptor for state field.
	userDescState := userMixinFields1[4].Descriptor()
	// user.DefaultState holds the default value on creation for the state field.
	user.DefaultState = userDescState.Default.(string)
	// user.StateValidator is a validator for the "state" field. It is called by the builders before save.
	user.StateValidator = userDescState.Validators[0].(func(string) error)
	// userDescEmail is the schema descriptor for email field.
	userDescEmail := userFields[1].Descriptor()
	// user.EmailValidator is a validator for the "email" field. It is called by the builders before save.
	user.EmailValidator = userDescEmail.Validators[0].(func(string) error)
	// userDescOauthID is the schema descriptor for oauth_id field.
	userDescOauthID := userFields[3].Descriptor()
	// user.OauthIDValidator is a validator for the "oauth_id" field. It is called by the builders before save.
	user.OauthIDValidator = userDescOauthID.Validators[0].(func(string) error)
	// userDescFirstName is the schema descriptor for first_name field.
	userDescFirstName := userFields[5].Descriptor()
	// user.DefaultFirstName holds the default value on creation for the first_name field.
	user.DefaultFirstName = userDescFirstName.Default.(string)
	// user.FirstNameValidator is a validator for the "first_name" field. It is called by the builders before save.
	user.FirstNameValidator = userDescFirstName.Validators[0].(func(string) error)
	// userDescLastName is the schema descriptor for last_name field.
	userDescLastName := userFields[6].Descriptor()
	// user.DefaultLastName holds the default value on creation for the last_name field.
	user.DefaultLastName = userDescLastName.Default.(string)
	// user.LastNameValidator is a validator for the "last_name" field. It is called by the builders before save.
	user.LastNameValidator = userDescLastName.Validators[0].(func(string) error)
	// userDescPhoneNumber is the schema descriptor for phone_number field.
	userDescPhoneNumber := userFields[7].Descriptor()
	// user.PhoneNumberValidator is a validator for the "phone_number" field. It is called by the builders before save.
	user.PhoneNumberValidator = userDescPhoneNumber.Validators[0].(func(string) error)
	// userDescIsEmailVerified is the schema descriptor for is_email_verified field.
	userDescIsEmailVerified := userFields[9].Descriptor()
	// user.DefaultIsEmailVerified holds the default value on creation for the is_email_verified field.
	user.DefaultIsEmailVerified = userDescIsEmailVerified.Default.(bool)
	// userDescMarketingOptIn is the schema descriptor for marketing_opt_in field.
	userDescMarketingOptIn := userFields[10].Descriptor()
	// user.DefaultMarketingOptIn holds the default value on creation for the marketing_opt_in field.
	user.DefaultMarketingOptIn = userDescMarketingOptIn.Default.(bool)
}
