package portal_links

import (
	"context"

	"github.com/dchest/uniuri"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/portal_links/repo"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/services"
	"github.com/frain-dev/convoy/util"
	"github.com/jackc/pgx/v5/pgtype"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/oklog/ulid/v2"
)

type Service struct {
	logger log.StdLogger
	repo   repo.Querier
}

func New(logger log.StdLogger, db *pgxpool.Pool) *Service {
	return &Service{
		logger: logger,
		repo:   repo.New(db),
	}
}

func (s *Service) CreatePortalLink(ctx context.Context, projectId string, request *models.CreatePortalLinkRequest) (*datastore.PortalLink, error) {
	if err := request.Validate(); err != nil {
		return nil, &services.ServiceError{ErrMsg: err.Error()}
	}

	uid := ulid.Make().String()
	if util.IsStringEmpty(request.OwnerID) {
		request.OwnerID = uid
	}

	pl := repo.CreatePortalLinkParams{
		ID:                uid,
		ProjectID:         projectId,
		Name:              request.Name,
		Token:             uniuri.NewLen(24),
		OwnerID:           pgtype.Text{String: request.OwnerID, Valid: true},
		AuthType:          repo.ConvoyPortalAuthTypes(request.AuthType),
		CanManageEndpoint: pgtype.Bool{Bool: request.CanManageEndpoint, Valid: true},
	}

	err := s.repo.CreatePortalLink(ctx, pl)
	if err != nil {
		s.logger.WithError(err).Error("failed to create portal link")
		return nil, &services.ServiceError{ErrMsg: "failed to create portal link"}
	}

	return &datastore.PortalLink{
		UID:               pl.ID,
		Name:              pl.Name,
		ProjectID:         pl.ProjectID,
		Token:             pl.Token,
		OwnerID:           pl.OwnerID.String,
		CanManageEndpoint: pl.CanManageEndpoint.Bool,
	}, nil
}
