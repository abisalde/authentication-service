package model

type LoginResponse struct {
	Token        string `json:"token"`
	UserId       int64  `json:"userId"`
	Email        string `json:"email"`
	RefreshToken string `json:"refreshToken"`
}

type RegisterResponse struct {
	User    PublicUser `json:"user"`
	Message string     `json:"message"`
	OauthID *string    `json:"oauthId"`
}

type OAuthUserResponse struct {
	ID              string  `json:"id"`
	Email           string  `json:"email"`
	Name            *string `json:"name,omitempty"`
	FirstName       string
	LastName        string
	Link            string `json:"link"`
	IsEmailVerified bool   `json:"isEmailVerified"`
}
