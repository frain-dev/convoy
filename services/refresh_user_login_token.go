package services

import (
	"context"
	"errors"
	"time"

	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/auth/realm/jwt"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/license"
	log "github.com/frain-dev/convoy/pkg/logger"
)

type RefreshTokenService struct {
	UserRepo      datastore.UserRepository
	OrgMemberRepo datastore.OrganisationMemberRepository
	JWT           *jwt.Jwt
	Licenser      license.Licenser

	Data   *models.Token
	Logger log.Logger
}

// NewRefreshTokenService takes every dependency as a required parameter so the
// license gate wired below cannot be silently skipped by a struct literal that
// forgets Licenser or OrgMemberRepo (the same class of bug as the bootstrap
// nil-Licenser panic).
func NewRefreshTokenService(
	userRepo datastore.UserRepository,
	orgMemberRepo datastore.OrganisationMemberRepository,
	jwtClient *jwt.Jwt,
	licenser license.Licenser,
	data *models.Token,
	logger log.Logger,
) *RefreshTokenService {
	return &RefreshTokenService{
		UserRepo:      userRepo,
		OrgMemberRepo: orgMemberRepo,
		JWT:           jwtClient,
		Licenser:      licenser,
		Data:          data,
		Logger:        logger,
	}
}

func (u *RefreshTokenService) Run(ctx context.Context) (*jwt.Token, error) {
	isValid, err := u.JWT.ValidateAccessToken(u.Data.AccessToken)
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			expiry := time.Unix(isValid.Expiry, 0)
			gracePeriod := expiry.Add(time.Minute * 5)
			currentTime := time.Now()

			// We allow a window period from the moment the access token has
			// expired
			if currentTime.After(gracePeriod) {
				return nil, &ServiceError{ErrMsg: err.Error()}
			}
		} else {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}
	}

	verified, err := u.JWT.ValidateRefreshToken(u.Data.RefreshToken)
	if err != nil {
		return nil, &ServiceError{ErrMsg: err.Error()}
	}

	user, err := u.UserRepo.FindUserByID(ctx, verified.UserID)
	if err != nil {
		if errors.Is(err, datastore.ErrUserNotFound) {
			return nil, &ServiceError{ErrMsg: err.Error()}
		}

		u.Logger.ErrorContext(ctx, "failed to find user by id", "error", err)
		return nil, &ServiceError{Code: ErrCodeInternal, ErrMsg: "failed to find user by id", Err: err}
	}

	// Enforce the same single-user-mode license gate as login. Without this a
	// non-admin who authenticated before the license lapsed could refresh
	// indefinitely and keep full access. Multi-user (licensed) instances pass
	// unchanged, so this only closes the loophole login already blocks.
	canAccess, err := IsPrimaryInstanceAdmin(ctx, u.Licenser, u.OrgMemberRepo, u.UserRepo, user.UID)
	if err != nil {
		u.Logger.ErrorContext(ctx, "failed to evaluate license access on refresh", "error", err)
		return nil, &ServiceError{Code: ErrCodeInternal, ErrMsg: "failed to evaluate license access", Err: err}
	}
	if !canAccess {
		return nil, &ServiceError{
			Code:   ErrCodeLicenseExpired,
			ErrMsg: "License expired. Only the first organization administrator can access the system"}
	}

	token, err := u.JWT.GenerateToken(user)
	if err != nil {
		u.Logger.ErrorContext(ctx, "failed to generate token", "error", err)
		return nil, &ServiceError{Code: ErrCodeInternal, ErrMsg: "failed to generate token", Err: err}
	}

	err = u.JWT.BlacklistToken(verified, u.Data.RefreshToken)
	if err != nil {
		u.Logger.ErrorContext(ctx, "failed to blacklist token", "error", err)
		return nil, &ServiceError{Code: ErrCodeInternal, ErrMsg: "failed to blacklist token", Err: err}
	}

	return &token, nil
}
