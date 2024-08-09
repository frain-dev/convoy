package stats

import (
	"context"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"
	"net/http"
	"net/http/httptest"
	"net/url"
	"testing"
)

func TestStats_Record(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	dr := mocks.NewMockEventDeliveryRepository(ctrl)
	co := mocks.NewMockConfigurationRepository(ctrl)

	dr.EXPECT().CountInstanceEventDeliveries(gomock.Any()).Times(1).Return(uint64(4), nil)
	co.EXPECT().LoadConfiguration(gomock.Any()).Times(1).Return(&datastore.Configuration{
		UID: "123",
	}, nil)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, err := w.Write([]byte("OK"))
		require.Nil(t, err)
	}))
	serverURL, err := url.Parse(server.URL)
	require.NoError(t, err)

	st := NewStats(serverURL.String(), dr, co)

	err = st.Record(context.TODO())
	require.NoError(t, err)
}
