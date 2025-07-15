package model

import "time"

type User struct {
	ID              int64        `json:"id"`
	Email           string       `json:"email"`
	Provider        AuthProvider `json:"provider"`
	FirstName       string       `json:"firstName"`
	LastName        string       `json:"lastName"`
	CreatedAt       time.Time    `json:"createdAt"`
	UpdatedAt       time.Time    `json:"updatedAt"`
	DeletedAt       *time.Time   `json:"deletedAt"`
	OauthId         *string      `json:"oauthId"`
	Address         *UserAddress `json:"address"`
	PhoneNumber     string       `json:"phoneNumber"`
	Role            UserRole     `json:"role"`
	IsEmailVerified bool         `json:"isEmailVerified"`
	TermsAcceptedAt *time.Time   `json:"termsAcceptedAt"`
	MarketingOptIn  bool         `json:"marketingOptIn"`
	LastLoginAt     *time.Time   `json:"lastLoginAt"`
}

type PublicUser struct {
	Email string  `json:"email"`
	Name  *string `json:"name,omitempty"`
}

type RegisterVerifiedUser struct {
	Email           string  `json:"email"`
	Password        string  `json:"password"`
	IsEmailVerified bool    `json:"isEmailVerified"`
	FirstName       string  `json:"firstName"`
	LastName        string  `json:"lastName"`
	OauthId         *string `json:"oauthId"`
}

type PendingUser struct {
	Email            string    `json:"email"`
	HashPassword     string    `json:"hash_password"`
	VerificationCode string    `json:"code"`
	CreatedAt        time.Time `json:"createdAt"`
	ExpiresAt        time.Time `json:"expiresAt"`
}
