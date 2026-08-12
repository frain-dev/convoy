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

// compare-and-delete so a late Release cannot clear a newer claim after TTL expiry.
var resendClaimReleaseScript = redis.NewScript(`
if redis.call("get", KEYS[1]) == ARGV[1] then
  return redis.call("del", KEYS[1])
end
return 0
`)

// ResendClaimStore serializes verification-email resends per user.
// TryClaim returns (true, token, nil) when the claim is acquired.
// Release must use the same token (compare-and-delete).
type ResendClaimStore interface {
	TryClaim(ctx context.Context, userUID string) (ok bool, token string, err error)
	Release(ctx context.Context, userUID, token string) error
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

func (s *redisResendClaimStore) TryClaim(ctx context.Context, userUID string) (bool, string, error) {
	token := ulid.Make().String()
	ok, err := s.rdb.SetNX(ctx, s.claimKey(userUID), token, emailVerificationResendCooldown).Result()
	if err != nil {
		return false, "", err
	}
	if !ok {
		return false, "", nil
	}
	return true, token, nil
}

func (s *redisResendClaimStore) Release(ctx context.Context, userUID, token string) error {
	return resendClaimReleaseScript.Run(ctx, s.rdb, []string{s.claimKey(userUID)}, token).Err()
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

	claimToken := ""
	if u.ClaimStore != nil {
		ok, token, err := u.ClaimStore.TryClaim(ctx, u.User.UID)
		if err != nil {
			// Failure policy: Redis transport errors fail open to soft cooldown
			// (already passed) so an outage does not block a legitimate resend.
			if u.Logger != nil {
				u.Logger.ErrorContext(ctx, "verification resend claim error; continuing with soft cooldown only", "error", err)
			}
		} else if !ok {
			return &ServiceError{ErrMsg: "please wait before requesting another verification email"}
		} else {
			claimToken = token
		}
	}

	prevToken := u.User.EmailVerificationToken
	prevExpiresAt := u.User.EmailVerificationExpiresAt

	newToken := ulid.Make().String()
	newExpiresAt := time.Now().Add(emailVerificationTokenTTL)

	// Token-only rotate with email_verified=false gate. Avoids full-row UpdateUser
	// racing VerifyEmailService (stale EmailVerified=false would undo verify).
	err := u.UserRepo.RotateEmailVerificationToken(ctx, u.User.UID, newToken, newExpiresAt)
	if err != nil {
		u.releaseClaim(ctx, claimToken)
		if u.alreadyVerified(ctx) {
			return &ServiceError{ErrMsg: "user email already verified"}
		}
		if u.Logger != nil {
			u.Logger.ErrorContext(ctx, "failed to rotate email verification token", "error", err)
		}
		return &ServiceError{ErrMsg: "failed to update user", Err: err}
	}

	u.User.EmailVerificationToken = newToken
	u.User.EmailVerificationExpiresAt = newExpiresAt

	err = sendUserVerificationEmail(ctx, u.BaseURL, u.User, u.Queue, u.Logger)
	if err != nil {
		// Restore the previous token so a still-valid link is not orphaned when
		// the new mail never left the queue. Failure policy: best-effort restore;
		// skip restore if the user verified concurrently; always release the claim
		// so the user can retry immediately.
		if !u.alreadyVerified(ctx) {
			if restoreErr := u.UserRepo.RotateEmailVerificationToken(ctx, u.User.UID, prevToken, prevExpiresAt); restoreErr != nil && u.Logger != nil {
				u.Logger.ErrorContext(ctx, "failed to restore previous verification token after queue error", "error", restoreErr)
			} else {
				u.User.EmailVerificationToken = prevToken
				u.User.EmailVerificationExpiresAt = prevExpiresAt
			}
		}
		u.releaseClaim(ctx, claimToken)
		return &ServiceError{ErrMsg: "failed to queue user verification email", Err: err}
	}

	return nil
}

// alreadyVerified re-reads the user after a rotate that affected 0 rows (verify won).
func (u *ResendEmailVerificationTokenService) alreadyVerified(ctx context.Context) bool {
	fresh, err := u.UserRepo.FindUserByID(ctx, u.User.UID)
	if err != nil {
		return false
	}
	if fresh.EmailVerified {
		u.User.EmailVerified = true
		return true
	}
	return false
}

func (u *ResendEmailVerificationTokenService) releaseClaim(ctx context.Context, token string) {
	if token == "" || u.ClaimStore == nil {
		return
	}
	if err := u.ClaimStore.Release(ctx, u.User.UID, token); err != nil && u.Logger != nil {
		u.Logger.ErrorContext(ctx, "failed to release verification resend claim", "error", err)
	}
}
