package resolvers

import (
	"github.com/abisalde/authentication-service/internal/auth/handler/http"
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
}

func NewResolver(client *ent.Client, authService service.AuthService) *Resolver {
	registerHandler := http.NewRegisterHandler(authService)
	loginHandler := http.NewLoginHandler(authService)
	profileHandler := http.NewProfileHandler(authService)
	return &Resolver{
		client:          client,
		registerHandler: registerHandler,
		loginHandler:    loginHandler,
		profileHandler:  profileHandler,
	}
}
