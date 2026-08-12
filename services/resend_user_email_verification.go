package services

import (
	"context"
	"fmt"
	"time"

	"github.com/oklog/ulid/v2"
	"github.com/redis/go-redis/v9"

	"github.com/frain-dev/convoy/datastore"
	log "github.com/frain-dev/convoy/pkg/logger"
	"github.com/frain-dev/convoy/queue"
)

// emailVerificationTokenTTL is how long a verification token remains usable.
const emailVerificationTokenTTL = 2 * time.Hour

// emailVerificationResendCooldown is the minimum gap between verification emails.
const emailVerificationResendCooldown = time.Minute

// ResendClaimStore serializes verification-email resends per user.
// TryClaim returns false when another claim is held (cooldown active).
// Release clears a claim after a failed resend so the user can retry immediately.
type ResendClaimStore interface {
	TryClaim(ctx context.Context, userUID string) (bool, error)
	Release(ctx context.Context, userUID string) error
}

type redisResendClaimStore struct {
	rdb redis.UniversalClient
}

func NewRedisResendClaimStore(rdb redis.UniversalClient) ResendClaimStore {
	if rdb == nil {
		return nil
	}
	return &redisResendClaimStore{rdb: rdb}
}

func (s *redisResendClaimStore) claimKey(userUID string) string {
	return fmt.Sprintf("email_verification_resend:%s", userUID)
}

func (s *redisResendClaimStore) TryClaim(ctx context.Context, userUID string) (bool, error) {
	return s.rdb.SetNX(ctx, s.claimKey(userUID), "1", emailVerificationResendCooldown).Result()
}

func (s *redisResendClaimStore) Release(ctx context.Context, userUID string) error {
	return s.rdb.Del(ctx, s.claimKey(userUID)).Err()
}

type ResendEmailVerificationTokenService struct {
	UserRepo   datastore.UserRepository
	Queue      queue.Queuer
	ClaimStore ResendClaimStore

	BaseURL string
	User    *datastore.User
	Logger  log.Logger
}

func (u *ResendEmailVerificationTokenService) Run(ctx context.Context) error {
	if u.User.EmailVerified {
		return &ServiceError{ErrMsg: "user email already verified"}
	}

	// Soft cooldown from the last mint time (ExpiresAt - TTL). Not atomic alone;
	// ClaimStore below serializes concurrent resends when Redis is available.
	if !u.User.EmailVerificationExpiresAt.IsZero() {
		lastSent := u.User.EmailVerificationExpiresAt.Add(-emailVerificationTokenTTL)
		if time.Now().Before(lastSent.Add(emailVerificationResendCooldown)) {
			return &ServiceError{ErrMsg: "please wait before requesting another verification email"}
		}
	}

	claimed := false
	if u.ClaimStore != nil {
		ok, err := u.ClaimStore.TryClaim(ctx, u.User.UID)
		if err != nil {
			// Failure policy: Redis transport errors fail open to soft cooldown
			// (already passed) so an outage does not block a legitimate resend.
			if u.Logger != nil {
				u.Logger.ErrorContext(ctx, "verification resend claim error; continuing with soft cooldown only", "error", err)
			}
		} else if !ok {
			return &ServiceError{ErrMsg: "please wait before requesting another verification email"}
		} else {
			claimed = true
		}
	}

	prevToken := u.User.EmailVerificationToken
	prevExpiresAt := u.User.EmailVerificationExpiresAt

	u.User.EmailVerificationExpiresAt = time.Now().Add(emailVerificationTokenTTL)
	u.User.EmailVerificationToken = ulid.Make().String()

	err := u.UserRepo.UpdateUser(ctx, u.User)
	if err != nil {
		u.releaseClaim(ctx, claimed)
		if u.Logger != nil {
			u.Logger.ErrorContext(ctx, "failed to update user", "error", err)
		}
		return &ServiceError{ErrMsg: "failed to update user", Err: err}
	}

	err = sendUserVerificationEmail(ctx, u.BaseURL, u.User, u.Queue, u.Logger)
	if err != nil {
		// Restore the previous token so a still-valid link is not orphaned when
		// the new mail never left the queue. Failure policy: best-effort restore;
		// always release the claim so the user can retry immediately.
		u.User.EmailVerificationToken = prevToken
		u.User.EmailVerificationExpiresAt = prevExpiresAt
		if restoreErr := u.UserRepo.UpdateUser(ctx, u.User); restoreErr != nil && u.Logger != nil {
			u.Logger.ErrorContext(ctx, "failed to restore previous verification token after queue error", "error", restoreErr)
		}
		u.releaseClaim(ctx, claimed)
		return &ServiceError{ErrMsg: "failed to queue user verification email", Err: err}
	}

	return nil
}

func (u *ResendEmailVerificationTokenService) releaseClaim(ctx context.Context, claimed bool) {
	if !claimed || u.ClaimStore == nil {
		return
	}
	if err := u.ClaimStore.Release(ctx, u.User.UID); err != nil && u.Logger != nil {
		u.Logger.ErrorContext(ctx, "failed to release verification resend claim", "error", err)
	}
}
