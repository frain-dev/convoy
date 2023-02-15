package pubsub

import (
	"io"
	"testing"

	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/golang/mock/gomock"
	"github.com/stretchr/testify/require"
)

func provideSourceLoader(ctrl *gomock.Controller) *SourceLoader {
	endpointRepo := mocks.NewMockEndpointRepository(ctrl)
	sourceRepo := mocks.NewMockSourceRepository(ctrl)
	queue := mocks.NewMockQueuer(ctrl)
	sourcePool := provideSourcePool()
	log := log.NewLogger(io.Discard)

	sourceLoader := NewSourceLoader(endpointRepo, sourceRepo, queue, sourcePool, log)
	return sourceLoader
}

func TestSourceLoader_FetchSources(t *testing.T) {
	tests := []struct {
		name              string
		dbFn              func(sourceLoader *SourceLoader)
		page              int
		expectedPubSource int
	}{
		{
			name: "should_fetch_two_pub_sub_sources",
			dbFn: func(sourceLoader *SourceLoader) {
				so, _ := sourceLoader.sourceRepo.(*mocks.MockSourceRepository)
				gomock.InOrder(
					so.EXPECT().LoadSourcesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return([]datastore.Source{
							{
								UID:  "12345",
								Type: datastore.PubSubSource,
								PubSub: &datastore.PubSubConfig{
									Type: datastore.SqsPubSub,
									Sqs:  &datastore.SQSPubSubConfig{},
								},
							},
							{
								UID:  "123456",
								Type: datastore.PubSubSource,
								PubSub: &datastore.PubSubConfig{
									Type:   datastore.GooglePubSub,
									Google: &datastore.GooglePubSubConfig{},
								},
							},
						}, datastore.PaginationData{}, nil),

					so.EXPECT().LoadSourcesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return([]datastore.Source{}, datastore.PaginationData{}, nil))
			},
			page:              1,
			expectedPubSource: 2,
		},

		{
			name: "should_fetch_one_pub_sub_source",
			dbFn: func(sourceLoader *SourceLoader) {
				so, _ := sourceLoader.sourceRepo.(*mocks.MockSourceRepository)
				gomock.InOrder(
					so.EXPECT().LoadSourcesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return([]datastore.Source{
							{
								UID:  "12345",
								Type: datastore.PubSubSource,
								PubSub: &datastore.PubSubConfig{
									Type: datastore.SqsPubSub,
									Sqs:  &datastore.SQSPubSubConfig{},
								},
							},
						}, datastore.PaginationData{}, nil),

					so.EXPECT().LoadSourcesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
						Return([]datastore.Source{}, datastore.PaginationData{}, nil))
			},
			page:              1,
			expectedPubSource: 1,
		},

		{
			name: "should_not_fetch_pub_sub_source",
			dbFn: func(sourceLoader *SourceLoader) {
				so, _ := sourceLoader.sourceRepo.(*mocks.MockSourceRepository)
				so.EXPECT().LoadSourcesPaged(gomock.Any(), gomock.Any(), gomock.Any(), gomock.Any()).
					Return([]datastore.Source{}, datastore.PaginationData{}, nil)
			},
			page: 1,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			ctrl := gomock.NewController(t)
			defer ctrl.Finish()

			sourceLoader := provideSourceLoader(ctrl)

			if tc.dbFn != nil {
				tc.dbFn(sourceLoader)
			}

			err := sourceLoader.fetchSources(tc.page)

			require.Nil(t, err)
			require.Equal(t, tc.expectedPubSource, len(sourceLoader.sourcePool.sources))
		})
	}
}
