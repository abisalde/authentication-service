package resolvers

import (
	"github.com/abisalde/authentication-service/internal/auth/handler/http"
	"github.com/abisalde/authentication-service/internal/auth/handler/oauth"
	"github.com/abisalde/authentication-service/internal/auth/service"
	"github.com/abisalde/authentication-service/internal/database/ent"
)

// This file will not be regenerated automatically.
//
// It serves as dependency injection for your app, add any dependencies you require here.

type Resolver struct {
	client          *ent.Client
	registerHandler *http.RegisterHandler
	loginHandler    *http.LoginHandler
	profileHandler  *http.ProfileHandler
	tokenHandler    *http.TokenHandler
	oauthHandler    *oauth.OAuthHandler
	usersHandler    *http.UsersHandler
	authService     service.AuthService
}

func NewResolver(client *ent.Client, authService service.AuthService, oauthService service.OAuthService) *Resolver {
	registerHandler := http.NewRegisterHandler(authService)
	loginHandler := http.NewLoginHandler(authService)
	profileHandler := http.NewProfileHandler(authService)
	usersHandler := http.NewUsersHandler(authService)
	tokenHandler := http.NewTokenHandler(authService)
	oauthHandler := oauth.NewOAuthHandler(&oauthService)
	return &Resolver{
		client:          client,
		registerHandler: registerHandler,
		loginHandler:    loginHandler,
		profileHandler:  profileHandler,
		usersHandler:    usersHandler,
		oauthHandler:    oauthHandler,
		tokenHandler:    tokenHandler,
		authService:     authService,
	}
}
