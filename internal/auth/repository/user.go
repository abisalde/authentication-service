package repository

import (
	"context"
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
}

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
