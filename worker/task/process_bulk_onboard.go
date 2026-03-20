package task

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/hibiken/asynq"
	"github.com/oklog/ulid/v2"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/fflag"
	"github.com/frain-dev/convoy/internal/pkg/license"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/util"
)

type BulkOnboardBatch struct {
	ProjectID string            `json:"project_id"`
	BatchID   string            `json:"batch_id"`
	Items     []BulkOnboardItem `json:"items"`
}

type BulkOnboardItem struct {
	Name         string `json:"name"`
	URL          string `json:"url"`
	EventType    string `json:"event_type"`
	AuthUsername string `json:"auth_username"`
	AuthPassword string `json:"auth_password"`
}

type BulkOnboardDeps struct {
	EndpointRepo               datastore.EndpointRepository
	SubRepo                    datastore.SubscriptionRepository
	ProjectRepo                datastore.ProjectRepository
	Licenser                   license.Licenser
	FeatureFlag                *fflag.FFlag
	FeatureFlagFetcher         fflag.FeatureFlagFetcher
	EarlyAdopterFeatureFetcher fflag.EarlyAdopterFeatureFetcher
}

func ProcessBulkOnboard(deps BulkOnboardDeps) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var batch BulkOnboardBatch

		err := msgpack.DecodeMsgPack(t.Payload(), &batch)
		if err != nil {
			// Fallback to JSON
			err = json.Unmarshal(t.Payload(), &batch)
			if err != nil {
				log.FromContext(ctx).WithError(err).Error("failed to decode bulk onboard payload")
				return err
			}
		}

		project, err := deps.ProjectRepo.FetchProjectByID(ctx, batch.ProjectID)
		if err != nil {
			log.FromContext(ctx).WithError(err).Errorf("failed to fetch project %s for bulk onboard", batch.ProjectID)
			return err
		}

		var successCount, failCount int
		for _, item := range batch.Items {
			// Check for existing endpoint with the same URL
			existingEndpoint, _ := deps.EndpointRepo.FindEndpointByTargetURL(ctx, project.UID, item.URL)
			var endpointID string

			if existingEndpoint != nil {
				log.FromContext(ctx).Warnf("bulk onboard: endpoint with URL %q already exists (uid=%s), skipping creation", item.URL, existingEndpoint.UID)
				endpointID = existingEndpoint.UID
			} else {
				endpoint, epErr := buildEndpoint(ctx, deps, project, item)
				if epErr != nil {
					log.FromContext(ctx).WithError(epErr).Errorf("bulk onboard: failed to build endpoint %q", item.Name)
					failCount++
					continue
				}

				epErr = deps.EndpointRepo.CreateEndpoint(ctx, endpoint, project.UID)
				if epErr != nil {
					log.FromContext(ctx).WithError(epErr).Errorf("bulk onboard: failed to create endpoint %q", item.Name)
					failCount++
					continue
				}
				endpointID = endpoint.UID
			}

			// Enforce MultipleEndpointSubscriptions setting
			if !project.Config.MultipleEndpointSubscriptions {
				count, countErr := deps.SubRepo.CountEndpointSubscriptions(ctx, project.UID, endpointID, "")
				if countErr != nil {
					log.FromContext(ctx).WithError(countErr).Errorf("bulk onboard: failed to count subscriptions for endpoint %q", item.Name)
					failCount++
					continue
				}
				if count > 0 {
					log.FromContext(ctx).Warnf("bulk onboard: subscription already exists for endpoint %s, skipping (MultipleEndpointSubscriptions disabled)", endpointID)
					successCount++
					continue
				}
			}

			subscription := buildSubscription(project.UID, endpointID, item, deps.Licenser)
			subErr := deps.SubRepo.CreateSubscription(ctx, project.UID, subscription)
			if subErr != nil {
				log.FromContext(ctx).WithError(subErr).Errorf("bulk onboard: failed to create subscription for endpoint %q", item.Name)
				failCount++
				continue
			}
			successCount++
		}

		if successCount == 0 && failCount > 0 {
			return fmt.Errorf("bulk onboard batch %s: all %d items failed", batch.BatchID, failCount)
		}

		return nil
	}
}

func buildEndpoint(ctx context.Context, deps BulkOnboardDeps, project *datastore.Project, item BulkOnboardItem) (*datastore.Endpoint, error) {
	uid := ulid.Make().String()

	advancedSignatures := true

	sc, err := util.GenerateSecret()
	if err != nil {
		return nil, err
	}

	endpoint := &datastore.Endpoint{
		UID:                uid,
		ProjectID:          project.UID,
		Name:               item.Name,
		Url:                item.URL,
		AppID:              uid,
		Status:             datastore.ActiveEndpointStatus,
		ContentType:        "application/json",
		AdvancedSignatures: advancedSignatures,
		Secrets: []datastore.Secret{
			{
				UID:       ulid.Make().String(),
				Value:     sc,
				CreatedAt: time.Now(),
				UpdatedAt: time.Now(),
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}

	if !deps.Licenser.AdvancedEndpointMgmt() {
		endpoint.HttpTimeout = convoy.HTTP_TIMEOUT
	}

	// Set basic auth if both username and password provided
	if item.AuthUsername != "" && item.AuthPassword != "" {
		if deps.Licenser.OAuth2EndpointAuth() {
			basicAuthEnabled := deps.FeatureFlag.CanAccessOrgFeature(
				ctx, fflag.BasicAuthEndpoint, deps.FeatureFlagFetcher, deps.EarlyAdopterFeatureFetcher, project.OrganisationID)
			if basicAuthEnabled {
				endpoint.Authentication = &datastore.EndpointAuthentication{
					Type: datastore.BasicAuthentication,
					BasicAuth: &datastore.BasicAuth{
						UserName: item.AuthUsername,
						Password: item.AuthPassword,
					},
				}
			} else {
				log.FromContext(ctx).Warnf("bulk onboard: basic auth feature flag not enabled for org %s, skipping auth for endpoint %q",
					project.OrganisationID, item.Name)
			}
		} else {
			log.FromContext(ctx).Warnf("bulk onboard: basic auth not licensed, skipping auth for endpoint %q", item.Name)
		}
	}

	return endpoint, nil
}

func buildSubscription(projectID, endpointID string, item BulkOnboardItem, licenser license.Licenser) *datastore.Subscription {
	eventType := item.EventType
	if eventType == "" {
		eventType = "*"
	}

	if !licenser.AdvancedSubscriptions() && eventType != "*" {
		log.Warnf("bulk onboard: advanced subscriptions not licensed, ignoring event type filter %q", eventType)
		eventType = "*"
	}

	return &datastore.Subscription{
		UID:          ulid.Make().String(),
		ProjectID:    projectID,
		Name:         fmt.Sprintf("%s-%s-subscription", item.Name, endpointID[:8]),
		Type:         datastore.SubscriptionTypeAPI,
		EndpointID:   endpointID,
		DeliveryMode: datastore.AtLeastOnceDeliveryMode,
		FilterConfig: &datastore.FilterConfiguration{
			EventTypes: []string{eventType},
			Filter: datastore.FilterSchema{
				Headers:    datastore.M{},
				Body:       datastore.M{},
				RawHeaders: datastore.M{},
				RawBody:    datastore.M{},
			},
		},
		CreatedAt: time.Now(),
		UpdatedAt: time.Now(),
	}
}
