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
