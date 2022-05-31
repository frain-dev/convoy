package mongo

import (
	"context"
	"errors"

	"github.com/frain-dev/convoy/datastore"
	pager "github.com/gobeam/mongo-go-pagination"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
)

type userRepo struct {
	innerDB *mongo.Database
	client  *mongo.Collection
}

func NewUserRepo(db *mongo.Database) datastore.UserRepository {
	return &userRepo{
		innerDB: db,
		client:  db.Collection(UserCollection),
	}
}

func (u *userRepo) CreateUser(ctx context.Context, user *datastore.User) error {
	user.ID = primitive.NewObjectID()

	_, err := u.client.InsertOne(ctx, user)
	return err
}

func (u *userRepo) FindUserByEmail(ctx context.Context, email string) (*datastore.User, error) {
	user := &datastore.User{}

	filter := bson.M{"email": email, "document_status": datastore.ActiveDocumentStatus}

	err := u.client.FindOne(ctx, filter).Decode(&user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return user, datastore.ErrUserNotFound
	}

	return user, nil
}

func (u *userRepo) FindUserByID(ctx context.Context, id string) (*datastore.User, error) {
	user := &datastore.User{}

	filter := bson.M{"uid": id, "document_status": datastore.ActiveDocumentStatus}

	err := u.client.FindOne(ctx, filter).Decode(&user)

	if errors.Is(err, mongo.ErrNoDocuments) {
		return user, datastore.ErrUserNotFound
	}

	return user, nil
}

func (u *userRepo) LoadUsersPaged(ctx context.Context, pageable datastore.Pageable) ([]datastore.User, datastore.PaginationData, error) {
	var users []datastore.User

	filter := bson.M{"document_status": datastore.ActiveDocumentStatus}

	paginatedData, err := pager.New(u.client).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", -1).Filter(filter).Decode(&users).Find()
	if err != nil {
		return users, datastore.PaginationData{}, err
	}

	if users == nil {
		users = make([]datastore.User, 0)
	}

	return users, datastore.PaginationData(paginatedData.Pagination), nil
}
