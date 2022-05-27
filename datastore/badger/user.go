package badger

import (
	"context"

	"github.com/frain-dev/convoy/datastore"
	"github.com/timshannon/badgerhold/v4"
)

type userRepo struct {
	db *badgerhold.Store
}

func NewUserRepo(db *badgerhold.Store) datastore.UserRepository {
	return &userRepo{db: db}
}

func (u *userRepo) CreateUser(ctx context.Context, user *datastore.User) error {
	return nil
}

func (u *userRepo) FindUserByEmail(ctx context.Context, email string) (*datastore.User, error) {
	return nil, nil
}

func (u *userRepo) FindUserByID(ctx context.Context, id string) (*datastore.User, error) {
	return nil, nil
}

func (u *userRepo) LoadUsersPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.User, datastore.PaginationData, error) {
	return nil, datastore.PaginationData{}, nil
}
