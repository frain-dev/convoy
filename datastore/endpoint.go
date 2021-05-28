package datastore

import (
	"go.mongodb.org/mongo-driver/mongo"
	"gorm.io/gorm"
)

type endpointDB struct {
	inner  *gorm.DB
	client *mongo.Client
}

// func NewEndpointRepository(db *mongo.Client) hookcamp.EndpointRepository {
// 	return &endpointDB{
// 		// inner: db,
// 		client: db,
// 	}
// }

// func (e *endpointDB) CreateEndpoint(ctx context.Context,
// 	endpoint *hookcamp.Endpoint) error {
// 	if endpoint.ID == uuid.Nil {
// 		endpoint.ID = uuid.New()
// 	}

// 	return e.inner.WithContext(ctx).
// 		Create(endpoint).
// 		Error
// }

// func (e *endpointDB) FindEndpointByID(ctx context.Context,
// 	id uuid.UUID) (*hookcamp.Endpoint, error) {
// 	app := new(hookcamp.Endpoint)

// 	err := e.inner.WithContext(ctx).
// 		Where(&hookcamp.Endpoint{ID: id}).
// 		First(app).
// 		Error

// 	if errors.Is(err, gorm.ErrRecordNotFound) {
// 		err = hookcamp.ErrEndpointNotFound
// 	}

// 	return app, err
// }
