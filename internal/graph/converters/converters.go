package converters

import (
	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/abisalde/authentication-service/internal/graph/model"
)

func UserToGraph(user *ent.User) *model.User {
	var username *string
	if user.Username != "" {
		username = &user.Username
	}

	return &model.User{
		ID:              user.ID,
		Email:           user.Email,
		Username:        username,
		Provider:        model.AuthProvider(user.Provider),
		FirstName:       user.FirstName,
		LastName:        user.LastName,
		CreatedAt:       user.CreatedAt,
		UpdatedAt:       user.UpdatedAt,
		OauthId:         &user.OauthID,
		Address: &model.UserAddress{
			StreetName: &user.StreetName,
			City:       &user.City,
			ZipCode:    &user.ZipCode,
			Country:    &user.Country,
			State:      &user.State,
		},
		PhoneNumber:     user.PhoneNumber,
		IsEmailVerified: user.IsEmailVerified,
		TermsAcceptedAt: user.TermsAcceptedAt,
		MarketingOptIn:  user.MarketingOptIn,
		LastLoginAt:     user.LastLoginAt,
	}
}
