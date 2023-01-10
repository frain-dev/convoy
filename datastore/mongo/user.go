package mongo

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/datastore"
	"github.com/google/uuid"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type userRepo struct {
	store datastore.Store
}

func NewUserRepo(store datastore.Store) datastore.UserRepository {
	return &userRepo{
		store: store,
	}
}

func (u *userRepo) CreateUser(ctx context.Context, user *datastore.User) error {
	ctx = u.setCollectionInContext(ctx)
	user.ID = primitive.NewObjectID()
	user.ResetPasswordToken = uuid.NewString()

	err := u.store.Save(ctx, user, nil)

	if mongo.IsDuplicateKeyError(err) {
		return datastore.ErrDuplicateEmail
	}

	return err
}

func (u *userRepo) FindUserByEmail(ctx context.Context, email string) (*datastore.User, error) {
	ctx = u.setCollectionInContext(ctx)
	user := &datastore.User{}

	filter := bson.M{"email": email}

	err := u.store.FindOne(ctx, filter, nil, user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return user, datastore.ErrUserNotFound
	}

	return user, nil
}

func (u *userRepo) FindUserByID(ctx context.Context, id string) (*datastore.User, error) {
	ctx = u.setCollectionInContext(ctx)
	user := &datastore.User{}

	err := u.store.FindByID(ctx, id, nil, user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return user, datastore.ErrUserNotFound
	}

	return user, nil
}

func (u *userRepo) LoadUsersPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.User, datastore.PaginationData, error) {
	ctx = u.setCollectionInContext(ctx)
	var users []datastore.User

	pagination, err := u.store.FindMany(ctx, bson.M{}, nil, nil,
		int64(pageable.Page), int64(pageable.PerPage), &users)
	if err != nil {
		return users, datastore.PaginationData{}, err
	}

	if users == nil {
		users = make([]datastore.User, 0)
	}

	return users, pagination, nil
}

func (u *userRepo) UpdateUser(ctx context.Context, user *datastore.User) error {
	ctx = u.setCollectionInContext(ctx)
	update := bson.D{
		primitive.E{Key: "first_name", Value: user.FirstName},
		primitive.E{Key: "last_name", Value: user.LastName},
		primitive.E{Key: "email", Value: user.Email},
		primitive.E{Key: "password", Value: user.Password},
		primitive.E{Key: "email_verified", Value: user.EmailVerified},
		primitive.E{Key: "updated_at", Value: primitive.NewDateTimeFromTime(time.Now())},
		primitive.E{Key: "reset_password_token", Value: user.ResetPasswordToken},
		primitive.E{Key: "reset_password_expires_at", Value: user.ResetPasswordExpiresAt},
		primitive.E{Key: "email_verification_token", Value: user.EmailVerificationToken},
		primitive.E{Key: "email_verification_expires_at", Value: user.EmailVerificationExpiresAt},
	}

	err := u.store.UpdateByID(ctx, user.UID, bson.M{"$set": update})

	if mongo.IsDuplicateKeyError(err) {
		return datastore.ErrDuplicateEmail
	}
	return err
}

func (u *userRepo) FindUserByToken(ctx context.Context, token string) (*datastore.User, error) {
	ctx = u.setCollectionInContext(ctx)
	user := &datastore.User{}

	filter := bson.M{"reset_password_token": token}

	err := u.store.FindOne(ctx, filter, nil, user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return user, datastore.ErrUserNotFound
	}

	return user, nil
}

func (u *userRepo) FindUserByEmailVerificationToken(ctx context.Context, token string) (*datastore.User, error) {
	ctx = u.setCollectionInContext(ctx)
	user := &datastore.User{}

	filter := bson.M{"email_verification_token": token}

	err := u.store.FindOne(ctx, filter, nil, user)
	if errors.Is(err, mongo.ErrNoDocuments) {
		return user, datastore.ErrUserNotFound
	}

	return user, nil
}

func (db *userRepo) setCollectionInContext(ctx context.Context) context.Context {
	return context.WithValue(ctx, datastore.CollectionCtx, datastore.UserCollection)
}
