package datastore

// type messageRepo struct {
// 	inner *gorm.DB
// }

// func NewMessageRepository(db *gorm.DB) hookcamp.MessageRepository {
// 	return &messageRepo{
// 		inner: db,
// 	}
// }

// func (e *messageRepo) CreateMessage(ctx context.Context,
// 	message *hookcamp.Message) error {
// 	if message.ID == uuid.Nil {
// 		message.ID = uuid.New()
// 	}

// 	return e.inner.WithContext(ctx).
// 		Create(message).
// 		Error
// }
