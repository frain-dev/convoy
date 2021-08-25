package datastore

import (
	"context"
	"errors"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/google/uuid"
	"github.com/hookcamp/hookcamp"
	"github.com/hookcamp/hookcamp/server/models"
	"github.com/hookcamp/hookcamp/util"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type messageRepo struct {
	inner *mongo.Collection
}

const (
	MsgCollection = "messages"
)

func NewMessageRepository(db *mongo.Database) hookcamp.MessageRepository {
	return &messageRepo{
		inner: db.Collection(MsgCollection),
	}
}

func (db *messageRepo) CreateMessage(ctx context.Context,
	message *hookcamp.Message) error {

	message.ID = primitive.NewObjectID()

	if util.IsStringEmpty(message.ProviderID) {
		message.ProviderID = message.AppID
	}
	if util.IsStringEmpty(message.UID) {
		message.UID = uuid.New().String()
	}

	if message.MessageAttempts == nil {
		message.MessageAttempts = make([]hookcamp.MessageAttempt, 0)
	}

	_, err := db.inner.InsertOne(ctx, message)
	return err
}

func (db *messageRepo) LoadMessages(ctx context.Context, orgId string, searchParams models.SearchParams) ([]hookcamp.Message, error) {

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd
	if end == 0 || end < searchParams.CreatedAtStart {
		end = start
	}

	findOptions := options.Find()
	findOptions.SetSort(bson.D{primitive.E{Key: "created_at", Value: -1}})

	log.Println("org_id", orgId)
	filter := bson.M{"application.org_id": orgId, "created_at": bson.M{"$gte": start, "$lte": end}}

	return db.loadMessagesByFilter(ctx, filter, findOptions)
}

func (db *messageRepo) LoadMessagesByAppId(ctx context.Context, appId string) ([]hookcamp.Message, error) {
	filter := bson.M{"app_id": appId}

	return db.loadMessagesByFilter(ctx, filter, nil)
}

func (db *messageRepo) FindMessageByID(ctx context.Context, id string) (*hookcamp.Message, error) {
	m := new(hookcamp.Message)

	filter := bson.D{
		primitive.E{
			Key:   "uid",
			Value: id,
		},
	}

	err := db.inner.FindOne(ctx, filter).
		Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = hookcamp.ErrMessageNotFound
	}

	return m, err
}

func (db *messageRepo) LoadMessagesScheduledForPosting(ctx context.Context) ([]hookcamp.Message, error) {

	filter := bson.M{"status": hookcamp.ScheduledMessageStatus}

	return db.loadMessagesByFilter(ctx, filter, nil)
}

func (db *messageRepo) loadMessagesByFilter(ctx context.Context, filter bson.M, findOptions *options.FindOptions) ([]hookcamp.Message, error) {
	messages := make([]hookcamp.Message, 0)
	cur, err := db.inner.Find(ctx, filter, findOptions)
	if err != nil {
		return messages, err
	}

	for cur.Next(ctx) {
		var message hookcamp.Message
		if err := cur.Decode(&message); err != nil {
			return messages, err
		}

		messages = append(messages, message)
	}

	if err := cur.Err(); err != nil {
		return nil, err
	}

	if err := cur.Close(ctx); err != nil {
		return messages, err
	}

	return messages, nil
}

func (db *messageRepo) LoadMessagesForPostingRetry(ctx context.Context) ([]hookcamp.Message, error) {

	filter := bson.M{
		"$and": []bson.M{
			{"status": hookcamp.RetryMessageStatus},
			{"metadata.next_send_time": bson.M{"$lte": time.Now().Unix()}},
		},
	}

	return db.loadMessagesByFilter(ctx, filter, nil)
}

func (db *messageRepo) LoadAbandonedMessagesForPostingRetry(ctx context.Context) ([]hookcamp.Message, error) {
	filter := bson.M{
		"$and": []bson.M{
			{"status": hookcamp.ProcessingMessageStatus},
			{"metadata.next_send_time": bson.M{"$lte": time.Now().Unix()}},
		},
	}

	return db.loadMessagesByFilter(ctx, filter, nil)
}

func (db *messageRepo) UpdateStatusOfMessages(ctx context.Context, messages []hookcamp.Message, status hookcamp.MessageStatus) error {

	filter := bson.M{"uid": bson.M{"$in": getIds(messages)}}
	update := bson.M{"$set": bson.M{"status": status, "updated_at": time.Now().Unix()}}

	_, err := db.inner.UpdateMany(
		ctx,
		filter,
		update,
	)
	if err != nil {
		log.Errorln("error updating many messages - ", err)
		return err
	}

	return nil
}

func getIds(messages []hookcamp.Message) []string {
	ids := make([]string, 0)
	for _, message := range messages {
		ids = append(ids, message.UID)
	}
	return ids
}

func (db *messageRepo) UpdateMessage(ctx context.Context, m hookcamp.Message) error {
	filter := bson.M{"uid": m.UID}

	update := bson.M{
		"$set": bson.M{
			"status":      m.Status,
			"description": m.Description,
			"application": m.Application,
			"metadata":    m.Metadata,
			"attempts":    m.MessageAttempts,
			"updated_at":  time.Now().Unix(),
		},
	}

	_, err := db.inner.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Errorf("error updating one message %s - %s\n", m.UID, err)
		return err
	}

	return err
}

func (db *messageRepo) LoadMessagesPaged(ctx context.Context, pageable models.Pageable) ([]hookcamp.Message, pager.PaginationData, error) {
	filter := bson.D{
		//primitive.E{
		//	Key:   "org_id",
		//	Value: orgId,
		//},
	} // TODO: sort for organisation

	var messages []hookcamp.Message
	paginatedData, err := pager.New(db.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", -1).Filter(filter).Decode(&messages).Find()
	if err != nil {
		return messages, pager.PaginationData{}, err
	}

	if messages == nil {
		messages = make([]hookcamp.Message, 0)
	}

	return messages, paginatedData.Pagination, nil
}
