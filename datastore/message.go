package datastore

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/server/models"
	"github.com/frain-dev/convoy/util"
	pager "github.com/gobeam/mongo-go-pagination"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

type messageRepo struct {
	inner *mongo.Collection
}

const (
	MsgCollection = "messages"
)

func NewMessageRepository(db *mongo.Database) convoy.MessageRepository {
	return &messageRepo{
		inner: db.Collection(MsgCollection),
	}
}

var dailyIntervalFormat = "%Y-%m-%d" // 1 day
var weeklyIntervalFormat = "%Y-%m"   // 1 week
var monthlyIntervalFormat = "%Y-%m"  // 1 month
var yearlyIntervalFormat = "%Y"      // 1 month

func (db *messageRepo) CreateMessage(ctx context.Context,
	message *convoy.Message) error {

	message.ID = primitive.NewObjectID()

	if util.IsStringEmpty(message.ProviderID) {
		message.ProviderID = message.AppID
	}
	if util.IsStringEmpty(message.UID) {
		message.UID = uuid.New().String()
	}

	if message.MessageAttempts == nil {
		message.MessageAttempts = make([]convoy.MessageAttempt, 0)
	}

	_, err := db.inner.InsertOne(ctx, message)
	return err
}

func (db *messageRepo) LoadMessageIntervals(ctx context.Context, groupID string, searchParams models.SearchParams, period convoy.Period, interval int) ([]models.MessageInterval, error) {

	start := searchParams.CreatedAtStart
	end := searchParams.CreatedAtEnd
	if end == 0 || end < searchParams.CreatedAtStart {
		end = start
	}

	matchStage := bson.D{{Key: "$match", Value: bson.D{
		{Key: "app_metadata.group_id", Value: groupID},
		{Key: "document_status", Value: bson.D{
			{Key: "$ne", Value: convoy.DeletedDocumentStatus},
		}},
		{Key: "created_at", Value: bson.D{
			{Key: "$gte", Value: primitive.NewDateTimeFromTime(time.Unix(start, 0))},
			{Key: "$lte", Value: primitive.NewDateTimeFromTime(time.Unix(end, 0))},
		},
		}},
	}}

	var timeComponent string
	var format string
	switch period {
	case convoy.Daily:
		timeComponent = "$dayOfYear"
		format = dailyIntervalFormat
	case convoy.Weekly:
		timeComponent = "$week"
		format = weeklyIntervalFormat
	case convoy.Monthly:
		timeComponent = "$month"
		format = monthlyIntervalFormat
	case convoy.Yearly:
		timeComponent = "$year"
		format = yearlyIntervalFormat
	default:
		return nil, errors.New("specified data cannot be generated for period")
	}
	groupStage := bson.D{
		{Key: "$group", Value: bson.D{
			{Key: "_id",
				Value: bson.D{
					{Key: "total_time",
						Value: bson.D{{Key: "$dateToString", Value: bson.D{{Key: "date", Value: "$created_at"}, {Key: "format", Value: format}}}},
					},
					{Key: "index", Value: bson.D{{Key: "$trunc", Value: bson.D{{Key: "$divide", Value: bson.A{
						bson.D{{Key: timeComponent, Value: "$created_at"}},
						interval,
					},
					}}}}},
				},
			},
			{Key: "count", Value: bson.D{{Key: "$sum", Value: 1}}},
		},
		},
	}
	sortStage := bson.D{{Key: "$sort", Value: bson.D{primitive.E{Key: "_id", Value: 1}}}}

	data, err := db.inner.Aggregate(ctx, mongo.Pipeline{matchStage, groupStage, sortStage})
	if err != nil {
		log.WithError(err).Errorln("aggregate error")
		return nil, err
	}
	var messagesIntervals []models.MessageInterval
	if err = data.All(ctx, &messagesIntervals); err != nil {
		log.WithError(err).Error("marshal error")
		return nil, err
	}
	if messagesIntervals == nil {
		messagesIntervals = make([]models.MessageInterval, 0)
	}

	return messagesIntervals, nil
}

func (db *messageRepo) LoadMessagesPagedByAppId(ctx context.Context, appId string, searchParams models.SearchParams, pageable models.Pageable) ([]convoy.Message, pager.PaginationData, error) {
	filter := bson.M{"app_id": appId, "document_status": bson.M{"$ne": convoy.DeletedDocumentStatus}, "created_at": getCreatedDateFilter(searchParams)}

	var messages []convoy.Message
	paginatedData, err := pager.New(db.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", pageable.Sort).Filter(filter).Decode(&messages).Find()
	if err != nil {
		return messages, pager.PaginationData{}, err
	}

	if messages == nil {
		messages = make([]convoy.Message, 0)
	}

	return messages, paginatedData.Pagination, nil
}

func (db *messageRepo) FindMessageByID(ctx context.Context, id string) (*convoy.Message, error) {
	m := new(convoy.Message)

	filter := bson.M{"uid": id, "document_status": bson.M{"$ne": convoy.DeletedDocumentStatus}}

	err := db.inner.FindOne(ctx, filter).
		Decode(&m)
	if errors.Is(err, mongo.ErrNoDocuments) {
		err = convoy.ErrMessageNotFound
	}

	return m, err
}

func (db *messageRepo) LoadMessagesScheduledForPosting(ctx context.Context) ([]convoy.Message, error) {

	filter := bson.M{"status": convoy.ScheduledMessageStatus,
		"document_status":         bson.M{"$ne": convoy.DeletedDocumentStatus},
		"metadata.next_send_time": bson.M{"$lte": primitive.NewDateTimeFromTime(time.Now())}}

	return db.loadMessagesByFilter(ctx, filter, nil)
}

func (db *messageRepo) loadMessagesByFilter(ctx context.Context, filter bson.M, findOptions *options.FindOptions) ([]convoy.Message, error) {
	messages := make([]convoy.Message, 0)
	cur, err := db.inner.Find(ctx, filter, findOptions)
	if err != nil {
		return messages, err
	}

	for cur.Next(ctx) {
		var message convoy.Message
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

func (db *messageRepo) LoadMessagesForPostingRetry(ctx context.Context) ([]convoy.Message, error) {

	filter := bson.M{
		"$and": []bson.M{
			{"status": convoy.RetryMessageStatus},
			{"metadata.next_send_time": bson.M{"$lte": primitive.NewDateTimeFromTime(time.Now())}},
			{"document_status": bson.M{"$ne": convoy.DeletedDocumentStatus}},
		},
	}

	return db.loadMessagesByFilter(ctx, filter, nil)
}

func (db *messageRepo) LoadAbandonedMessagesForPostingRetry(ctx context.Context) ([]convoy.Message, error) {
	filter := bson.M{
		"$and": []bson.M{
			{"status": convoy.ProcessingMessageStatus},
			{"metadata.next_send_time": bson.M{"$lte": primitive.NewDateTimeFromTime(time.Now())}},
			{"document_status": bson.M{"$ne": convoy.DeletedDocumentStatus}},
		},
	}

	return db.loadMessagesByFilter(ctx, filter, nil)
}

func (db *messageRepo) UpdateStatusOfMessages(ctx context.Context, messages []convoy.Message, status convoy.MessageStatus) error {

	filter := bson.M{"uid": bson.M{"$in": getIds(messages)}}
	update := bson.M{"$set": bson.M{"status": status, "updated_at": primitive.NewDateTimeFromTime(time.Now())}}

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

func getIds(messages []convoy.Message) []string {
	ids := make([]string, 0)
	for _, message := range messages {
		ids = append(ids, message.UID)
	}
	return ids
}

func (db *messageRepo) UpdateMessageWithAttempt(ctx context.Context, m convoy.Message, attempt convoy.MessageAttempt) error {
	filter := bson.M{"uid": m.UID}

	update := bson.M{
		"$set": bson.M{
			"status":       m.Status,
			"description":  m.Description,
			"app_metadata": m.AppMetadata,
			"metadata":     m.Metadata,
			"updated_at":   primitive.NewDateTimeFromTime(time.Now()),
		},
		"$push": bson.M{
			"attempts": attempt,
		},
	}

	_, err := db.inner.UpdateOne(ctx, filter, update)
	if err != nil {
		log.Errorf("error updating one message %s - %s\n", m.UID, err)
		return err
	}

	return err
}

func (db *messageRepo) LoadMessagesPaged(ctx context.Context, groupID string, appId string, searchParams models.SearchParams, pageable models.Pageable) ([]convoy.Message, pager.PaginationData, error) {
	filter := bson.M{"document_status": bson.M{"$ne": convoy.DeletedDocumentStatus}, "created_at": getCreatedDateFilter(searchParams)}

	hasAppFilter := !util.IsStringEmpty(appId)
	hasGroupFilter := !util.IsStringEmpty(groupID)

	if hasAppFilter && hasGroupFilter {
		filter = bson.M{"app_metadata.group_id": groupID, "app_id": appId, "document_status": bson.M{"$ne": convoy.DeletedDocumentStatus},
			"created_at": getCreatedDateFilter(searchParams)}
	} else if hasAppFilter {
		filter = bson.M{"app_id": appId, "document_status": bson.M{"$ne": convoy.DeletedDocumentStatus},
			"created_at": getCreatedDateFilter(searchParams)}
	} else if hasGroupFilter {
		filter = bson.M{"app_metadata.group_id": groupID, "document_status": bson.M{"$ne": convoy.DeletedDocumentStatus},
			"created_at": getCreatedDateFilter(searchParams)}
	}

	var messages []convoy.Message
	paginatedData, err := pager.New(db.inner).Context(ctx).Limit(int64(pageable.PerPage)).Page(int64(pageable.Page)).Sort("created_at", pageable.Sort).Filter(filter).Decode(&messages).Find()
	if err != nil {
		return messages, pager.PaginationData{}, err
	}

	if messages == nil {
		messages = make([]convoy.Message, 0)
	}

	return messages, paginatedData.Pagination, nil
}

func getCreatedDateFilter(searchParams models.SearchParams) bson.M {
	return bson.M{"$gte": primitive.NewDateTimeFromTime(time.Unix(searchParams.CreatedAtStart, 0)), "$lte": primitive.NewDateTimeFromTime(time.Unix(searchParams.CreatedAtEnd, 0))}
}
