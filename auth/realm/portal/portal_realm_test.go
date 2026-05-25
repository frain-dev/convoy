package portal

import (
	"context"
	"testing"

	"github.com/stretchr/testify/require"
	"go.uber.org/mock/gomock"

	"github.com/frain-dev/convoy/auth"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/mocks"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func TestPortalRealmAuthenticateRejectsOwnerIDFallback(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockPortalLinkRepository(ctrl)
	realm := NewPortalRealm(repo, log.New("convoy", log.LevelError))

	ownerID := "owner-123"
	repo.EXPECT().
		GetPortalLinkByToken(gomock.Any(), ownerID).
		Times(1).
		Return(nil, datastore.ErrPortalLinkNotFound)
	repo.EXPECT().
		FindPortalLinksByOwnerID(gomock.Any(), gomock.Any()).
		Times(0)

	user, err := realm.Authenticate(context.Background(), &auth.Credential{
		Type:  auth.CredentialTypeToken,
		Token: ownerID,
	})

	require.Nil(t, user)
	require.ErrorIs(t, err, datastore.ErrPortalLinkNotFound)
}

func TestPortalRealmAuthenticateAcceptsExactStaticToken(t *testing.T) {
	ctrl := gomock.NewController(t)
	defer ctrl.Finish()

	repo := mocks.NewMockPortalLinkRepository(ctrl)
	realm := NewPortalRealm(repo, log.New("convoy", log.LevelError))

	token := "static-token"
	portalLink := &datastore.PortalLink{
		UID:      "portal-link-1",
		Token:    token,
		AuthType: datastore.PortalAuthTypeStaticToken,
	}
	repo.EXPECT().
		GetPortalLinkByToken(gomock.Any(), token).
		Times(1).
		Return(portalLink, nil)

	user, err := realm.Authenticate(context.Background(), &auth.Credential{
		Type:  auth.CredentialTypeToken,
		Token: token,
	})

	require.NoError(t, err)
	require.Equal(t, portalLink, user.PortalLink)
}
