package pubsub

import (
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideSourcePool(ctrl *gomock.Controller) *SourcePool {
	queue := mocks.NewMockQueuer(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)

	return NewSourcePool(queue, sourceRepo, endpointRepo)
}

func TestPubSub_InsertSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPubSub(ctrl)
	sourcePool := provideSourcePool(ctrl)

	client.EXPECT().Start()

	uid := "12345"
	sourcePool.Insert(&datastore.Source{
		UID: uid,
		PubSub: &datastore.PubSubConfig{
			Type: datastore.SqsPubSub,
			Sqs:  &datastore.SQSPubSubConfig{},
		},
	}, client)

	_, ok := sourcePool.sources[uid]
	require.Equal(t, 1, len(sourcePool.sources))
	require.True(t, ok)
}

func TestPubSub_InsertSource_NewConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPubSub(ctrl)
	sourcePool := provideSourcePool(ctrl)

	uid := "12345"
	sourcePool.sources[uid] = &Source{client: client, hash: "random-hash"}

	client.EXPECT().Stop()
	client.EXPECT().Start()

	sourcePool.Insert(&datastore.Source{
		UID: uid,
		PubSub: &datastore.PubSubConfig{
			Type: datastore.SqsPubSub,
			Sqs: &datastore.SQSPubSubConfig{
				SecretKey:   "random-secret-key",
				AccessKeyID: "random-access-key-id",
			},
		},
	}, client)

	_, ok := sourcePool.sources[uid]
	require.Equal(t, 1, len(sourcePool.sources))
	require.True(t, ok)
}

func TestPubSub_RemoveSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPubSub(ctrl)
	sourcePool := provideSourcePool(ctrl)

	uid := "12345"
	sourcePool.sources[uid] = &Source{client: client}

	client.EXPECT().Stop()
	sourcePool.Remove(uid)

	_, ok := sourcePool.sources[uid]
	require.Equal(t, 0, len(sourcePool.sources))
	require.False(t, ok)

}
