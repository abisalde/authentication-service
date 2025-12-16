package schema

import (
	"regexp"
	"time"

	"entgo.io/ent"
	"entgo.io/ent/schema/edge"
	"entgo.io/ent/schema/field"
	"entgo.io/ent/schema/index"
	"entgo.io/ent/schema/mixin"
)

type UserAddressMixin struct {
	mixin.Schema
}

type UserAddress struct {
	ent.Schema
}

func (UserAddressMixin) Fields() []ent.Field {
	return []ent.Field{
		field.String("street_name").
			MaxLen(100).Default("").
			StructTag(`json:"streetName"`),

		field.String("city").
			Default("").
			MaxLen(50),

		field.String("zip_code").
			Default("").
			MaxLen(20).
			StructTag(`json:"zipCode"`),

		field.String("country").
			Default("").
			MaxLen(160).
			StructTag(`json:"country"`),

		field.String("state").
			Default("").
			MaxLen(100).
			StructTag(`json:"state"`),
	}
}

type TimeMixin struct {
	mixin.Schema
}

func (TimeMixin) Fields() []ent.Field {
	return []ent.Field{
		field.Time("created_at").
			Immutable().
			Default(time.Now).
			StructTag(`json:"createdAt"`),

		field.Time("updated_at").
			Default(time.Now).
			UpdateDefault(time.Now).
			StructTag(`json:"updatedAt"`),

		field.Time("deleted_at").
			Optional().
			Nillable().
			StructTag(`json:"deletedAt"`),
	}
}

type User struct {
	ent.Schema
}

func (User) Mixin() []ent.Mixin {
	return []ent.Mixin{
		TimeMixin{},
		UserAddressMixin{},
	}
}

func (User) Fields() []ent.Field {
	return []ent.Field{
		field.Int64("id").
			Immutable().
			StorageKey("id"),

		field.String("email").
			Unique().
			Match(regexp.MustCompile(`^[a-zA-Z0-9._%+-]+@[a-zA-Z0-9.-]+\.[a-zA-Z]{2,}$`)),

		field.String("username").
			Optional().
			Unique().
			MinLen(1).
			MaxLen(30).
			Match(regexp.MustCompile(`^[\p{L}\p{N}_'-]+$`)),

		field.String("password_hash").
			Sensitive().
			Optional(),

		field.String("oauth_id").
			Optional().
			MaxLen(255).
			StructTag(`json:"oauthId"`),

		field.Enum("provider").
			Values("GOOGLE", "FACEBOOK", "EMAIL").
			Default("EMAIL"),

		field.String("first_name").
			MaxLen(50).Default("").
			StructTag(`json:"firstName"`),

		field.String("last_name").
			MaxLen(50).Default("").
			StructTag(`json:"lastName"`),

		field.String("phone_number").
			Optional().
			Match(regexp.MustCompile(`^\+?[0-9\-\s]+$`)).
			StructTag(`json:"phoneNumber"`),

		field.Enum("role").
			Values("ADMIN", "USER").
			Default("USER"),

		field.Bool("is_email_verified").
			Default(false).
			StructTag(`json:"isEmailVerified"`),

		field.Bool("marketing_opt_in").
			Default(false).
			StructTag(`json:"marketingOptIn"`),

		field.Time("terms_accepted_at").
			Optional().
			Nillable().
			StructTag(`json:"termsAcceptedAt"`),

		field.Time("last_login_at").
			Optional().
			Nillable().
			StructTag(`json:"lastLoginAt"`),
	}
}

// Edges of the User.
func (User) Edges() []ent.Edge {
	return []ent.Edge{
		edge.To("address", UserAddress.Type).
			Unique().
			StructTag(`json:"address"`),
	}
}

func (User) Indexes() []ent.Index {
	return []ent.Index{
		index.Fields("email"),
		index.Fields("username"),
		index.Fields("oauth_id", "provider").Unique(),
		index.Fields("last_login_at"),
		index.Fields("is_email_verified"),
	}
}
