package pubsub

import (
	"fmt"
	"io"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideSourcePool() *SourcePool {
	logger := log.NewLogger(io.Discard)
	return NewSourcePool(logger)
}

func TestPubSub_InsertSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPubSub(ctrl)
	sourcePool := provideSourcePool()

	client.EXPECT().Start()

	uid := "12345"
	ps := &PubSubSource{
		source: &datastore.Source{
			UID: uid,
		},
		client: client,
	}
	sourcePool.Insert(ps)

	_, ok := sourcePool.sources[uid]
	require.Equal(t, 1, len(sourcePool.sources))
	require.True(t, ok)
}

func TestPubSub_InsertSource_NewConfig(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPubSub(ctrl)
	sourcePool := provideSourcePool()

	uid := "12345"

	source := &datastore.Source{UID: uid}
	ps := &PubSubSource{
		source: source,
		client: client,
		hash:   "random-hash",
	}

	sourcePool.sources[uid] = ps

	ps.source.PubSub = &datastore.PubSubConfig{
		Type: datastore.SqsPubSub,
		Sqs: &datastore.SQSPubSubConfig{
			SecretKey:   "random-secret-key",
			AccessKeyID: "random-access-key-id",
		},
	}

	fmt.Printf("sourcePool >>>> %+v\n", sourcePool.sources[uid].source.PubSub.Sqs)

	client.EXPECT().Stop()
	client.EXPECT().Start()

	sourcePool.Insert(ps)
	_, ok := sourcePool.sources[uid]
	require.Equal(t, 1, len(sourcePool.sources))
	require.True(t, ok)
}

func TestPubSub_RemoveSource(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	client := mocks.NewMockPubSub(ctrl)
	sourcePool := provideSourcePool()

	uid := "12345"
	sourcePool.sources[uid] = &PubSubSource{
		source: &datastore.Source{
			UID: uid,
		},
		client: client,
	}

	client.EXPECT().Stop()
	sourcePool.Remove(uid)

	_, ok := sourcePool.sources[uid]
	require.Equal(t, 0, len(sourcePool.sources))
	require.False(t, ok)

}
