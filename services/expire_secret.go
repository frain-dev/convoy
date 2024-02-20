package services

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/frain-dev/convoy/pkg/msgpack"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/api/models"
	"github.com/frain-dev/convoy/cache"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/util"
	"github.com/guregu/null/v5"
	"github.com/oklog/ulid/v2"
)

type ExpireSecretService struct {
	Queuer       queue.Queuer
	Cache        cache.Cache
	EndpointRepo datastore.EndpointRepository
	ProjectRepo  datastore.ProjectRepository

	S        *models.ExpireSecret
	Endpoint *datastore.Endpoint
	Project  *datastore.Project
}

func (a *ExpireSecretService) Run(ctx context.Context) (*datastore.Endpoint, error) {
	// Expire current secret.
	idx, err := a.Endpoint.GetActiveSecretIndex()
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	expiresAt := time.Now().Add(time.Second * time.Duration(a.S.Expiration))
	a.Endpoint.Secrets[idx].ExpiresAt = null.TimeFrom(expiresAt)

	secret := a.Endpoint.Secrets[idx]

	// Enqueue for final deletion.
	body := struct {
		EndpointID string `json:"endpoint_id"`
		SecretID   string `json:"secret_id"`
		ProjectID  string `json:"project_id"`
	}{
		EndpointID: a.Endpoint.UID,
		SecretID:   secret.UID,
		ProjectID:  a.Project.UID,
	}

	bytes, err := msgpack.EncodeMsgPack(body)
	if err != nil {
		return nil, util.NewServiceError(http.StatusBadRequest, err)
	}

	job := &queue.Job{
		ID:      secret.UID,
		Payload: bytes,
		Delay:   time.Hour * time.Duration(a.S.Expiration),
	}

	taskName := convoy.ExpireSecretsProcessor
	err = a.Queuer.Write(taskName, convoy.DefaultQueue, job)
	if err != nil {
		log.Errorf("Error occurred sending new event to the queue %s", err)
	}

	// Generate new secret.
	newSecret := a.S.Secret
	if len(newSecret) == 0 {
		newSecret, err = util.GenerateSecret()
		if err != nil {
			return nil, util.NewServiceError(http.StatusBadRequest, fmt.Errorf(fmt.Sprintf("could not generate secret...%v", err.Error())))
		}
	}

	sc := datastore.Secret{
		UID:       ulid.Make().String(),
		Value:     newSecret,
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	a.Endpoint.Secrets = append(a.Endpoint.Secrets, sc)

	err = a.EndpointRepo.UpdateSecrets(ctx, a.Endpoint.UID, a.Project.UID, a.Endpoint.Secrets)
	if err != nil {
		log.Errorf("Error occurred expiring secret %s", err)
		return nil, util.NewServiceError(http.StatusBadRequest, errors.New("failed to expire endpoint secret"))
	}

	return a.Endpoint, nil
}
