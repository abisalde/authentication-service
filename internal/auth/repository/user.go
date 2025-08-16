package repository

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"github.com/abisalde/authentication-service/internal/database/ent"
	"github.com/abisalde/authentication-service/internal/database/ent/user"
	"github.com/abisalde/authentication-service/internal/graph/model"
)

type UserRepository interface {
	GetByEmail(ctx context.Context, email string) (*ent.User, error)
	GetByID(ctx context.Context, id int64) (*ent.User, error)
	CreateNewUser(ctx context.Context, input *model.RegisterVerifiedUser) (*ent.User, error)
	ExistsByEmail(ctx context.Context, email string) (bool, error)
	UpdateLoginTime(ctx context.Context, userID int64) error
	UpdateNewPassword(ctx context.Context, userID int64, passwordHash string) error
	FindByOAuthID(ctx context.Context, provider, oauthID string) (*ent.User, error)
	CreateUserFromOAuth(ctx context.Context, provider string, userInfo *model.OAuthUserResponse) (*ent.User, error)
	FindAllUsers(ctx context.Context, role *model.UserRole, pagination *model.PaginationInput) (*model.UserConnection, error)
}

const (
	defaultLimit = 50
	maxLimit     = 100
)

type userRepository struct {
	client *ent.Client
}

func NewUserRepository(client *ent.Client) UserRepository {
	return &userRepository{client: client}
}

func (r *userRepository) GetByEmail(ctx context.Context, email string) (*ent.User, error) {
	return r.client.User.
		Query().
		Where(user.EmailEQ(email)).
		Only(ctx)
}

func (r *userRepository) GetByID(ctx context.Context, id int64) (*ent.User, error) {
	return r.client.User.
		Query().
		Where(user.IDEQ(id)).
		Only(ctx)
}

func (r *userRepository) ExistsByEmail(ctx context.Context, email string) (bool, error) {
	return r.client.User.
		Query().
		Where(user.EmailEQ(email)).
		Exist(ctx)
}

func (r *userRepository) CreateNewUser(ctx context.Context, input *model.RegisterVerifiedUser) (*ent.User, error) {
	firstName := input.FirstName
	lastName := input.LastName
	create := r.client.User.
		Create().
		SetEmail(input.Email).
		SetPasswordHash(input.Password).
		SetNillableIsEmailVerified(&input.IsEmailVerified).
		SetNillableOauthID(input.OauthId).
		SetFirstName(firstName).
		SetLastName(lastName)

	return create.Save(ctx)
}

func (r *userRepository) UpdateLoginTime(ctx context.Context, userID int64) error {
	err := r.client.User.UpdateOneID(userID).
		SetLastLoginAt(time.Now()).
		SetUpdatedAt(time.Now()).Exec(ctx)
	return err
}

func (r *userRepository) UpdateNewPassword(ctx context.Context, userID int64, passwordHash string) error {
	err := r.client.User.UpdateOneID(userID).
		SetPasswordHash(passwordHash).
		SetUpdatedAt(time.Now()).Exec(ctx)

	return err
}

func (r *userRepository) FindByOAuthID(ctx context.Context, provider, oauthID string) (*ent.User, error) {
	return r.client.User.
		Query().
		Where(
			user.OauthIDEQ(oauthID),
			user.ProviderEQ(user.Provider(provider)),
		).
		Only(ctx)
}

func (r *userRepository) CreateUserFromOAuth(ctx context.Context, provider string, userInfo *model.OAuthUserResponse) (*ent.User, error) {
	firstName := userInfo.FirstName
	lastName := userInfo.LastName
	providerEnum := user.Provider(provider)
	emailVerified := true

	create := r.client.User.
		Create().
		SetEmail(userInfo.Email).
		SetNillableIsEmailVerified(&emailVerified).
		SetNillableOauthID(&userInfo.ID).
		SetFirstName(firstName).
		SetNillableProvider(&providerEnum).
		SetLastName(lastName)

	return create.Save(ctx)
}

func (r *userRepository) FindAllUsers(ctx context.Context, role *model.UserRole, pagination *model.PaginationInput) (*model.UserConnection, error) {

	limit, afterID, err := validatePagination(pagination)
	if err != nil {
		return &model.UserConnection{
			Edges:    []*model.UserEdge{},
			PageInfo: &model.PageInfo{EndCursor: nil, HasNextPage: false},
		}, fmt.Errorf("invalid pagination: %w", err)
	}

	query := r.client.User.Query().
		Order(ent.Desc(user.FieldID)).Limit(limit)

	if role != nil {
		query = applyFilters(query, role)
	}

	if afterID > 0 {
		query = query.Where(user.IDGT(afterID))
	}

	users, err := query.All(ctx)
	if err != nil {
		return nil, err
	}

	return buildUserConnection(users, limit), nil
}

func validatePagination(pagination *model.PaginationInput) (limit int, afterID int64, err error) {
	limit = defaultLimit
	if pagination != nil {
		if pagination.Limit != nil {
			limit = *pagination.Limit
			if limit > maxLimit {
				limit = maxLimit
			}
		}

		if pagination.After != nil {
			afterID, err = strconv.ParseInt(*pagination.After, 10, 64)
			if err != nil {
				return 0, 0, fmt.Errorf("invalid cursor: %w", err)
			}
		}
	}
	return limit, afterID, nil
}

func mapEntUserToModelUser(u *ent.User) *model.User {
	if u == nil {
		return nil
	}
	return &model.User{
		ID:              u.ID,
		Email:           u.Email,
		FirstName:       u.FirstName,
		LastName:        u.LastName,
		IsEmailVerified: u.IsEmailVerified,
		OauthId:         &u.OauthID,
		Provider:        model.AuthProvider(u.Provider),
		Role:            model.UserRole(u.Role),
		LastLoginAt:     u.LastLoginAt,
		CreatedAt:       u.CreatedAt,
		UpdatedAt:       u.UpdatedAt,
	}
}

func buildUserConnection(users []*ent.User, limit int) *model.UserConnection {
	var edges []*model.UserEdge
	var endCursor *string

	for _, u := range users {
		cursor := strconv.FormatInt(u.ID, 10)
		endCursor = &cursor
		edges = append(edges, &model.UserEdge{
			Node:   mapEntUserToModelUser(u),
			Cursor: cursor,
		})
	}

	return &model.UserConnection{
		Edges: edges,
		PageInfo: &model.PageInfo{
			EndCursor:   endCursor,
			HasNextPage: len(users) == limit,
		},
	}
}

func applyFilters(query *ent.UserQuery, role *model.UserRole) *ent.UserQuery {
	if role != nil {
		query = query.Where(user.RoleEQ(user.Role(*role)))
	}

	return query
}
