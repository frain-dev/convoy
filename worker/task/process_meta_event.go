package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/signature"
	"github.com/frain-dev/convoy/retrystrategies"
	"github.com/frain-dev/convoy/util"
	"github.com/hibiken/asynq"
)

var (
	ErrMetaEventDeliveryFailed = errors.New("meta event delivery failed")
)

type MetaEvent struct {
	MetaEventID string
	ProjectID   string
}

func ProcessMetaEvent(projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var data MetaEvent

		err := json.Unmarshal(t.Payload(), &data)
		if err != nil {
			log.WithError(err).Error("failed to unmarshal process process meta event payload")
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		metaEvent, err := metaEventRepo.FindMetaEventByID(ctx, data.ProjectID, data.MetaEventID)
		if err != nil {
			log.WithError(err).Error("failed to find meta event by id")
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		project, err := projectRepo.FetchProjectByID(ctx, data.ProjectID)
		if err != nil {
			log.WithError(err).Error("failed to find project by id")
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		switch metaEvent.Status {
		case datastore.ProcessingEventStatus, datastore.SuccessEventStatus:
			return nil
		}

		metaEvent.Status = datastore.ProcessingEventStatus
		err = metaEventRepo.UpdateMetaEvent(ctx, metaEvent.ProjectID, metaEvent)
		if err != nil {
			log.WithError(err).Error("failed to update meta event status")
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		delayDuration := retrystrategies.NewRetryStrategyFromMetadata(*metaEvent.Metadata).NextDuration(metaEvent.Metadata.NumTrials)

		resp, err := sendUrlRequest(project, metaEvent)
		metaEvent.Metadata.NumTrials++
		responseHeader := util.ConvertDefaultHeaderToCustomHeader(&resp.ResponseHeader)
		requestHeader := util.ConvertDefaultHeaderToCustomHeader(&resp.RequestHeader)

		metaEvent.Attempt = &datastore.MetaEventAttempt{
			ResponseHeader: *responseHeader,
			RequestHeader:  *requestHeader,
			ResponseData:   string(resp.Body),
		}

		if err != nil {
			log.WithError(err).Error("failed to dispatch request")
			metaEvent.Status = datastore.RetryEventStatus
			nextTime := time.Now().Add(delayDuration)
			metaEvent.Metadata.NextSendTime = nextTime

			if metaEvent.Metadata.NumTrials >= metaEvent.Metadata.RetryLimit {
				metaEvent.Status = datastore.FailureEventStatus
			}

			err = metaEventRepo.UpdateMetaEvent(ctx, project.UID, metaEvent)
			if err != nil {
				log.WithError(err).Error("failed to update meta event")
			}

			if metaEvent.Metadata.NumTrials < metaEvent.Metadata.RetryLimit {
				log.Errorf("%s next retry time meta events is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", metaEvent.UID, nextTime.Format(time.ANSIC), metaEvent.Metadata.Strategy, metaEvent.Metadata.IntervalSeconds, metaEvent.Metadata.NumTrials, metaEvent.Metadata.RetryLimit)
				return &EndpointError{Err: ErrMetaEventDeliveryFailed, delay: delayDuration}
			}

			return nil
		}

		metaEvent.Status = datastore.SuccessEventStatus
		err = metaEventRepo.UpdateMetaEvent(ctx, project.UID, metaEvent)
		if err != nil {
			log.WithError(err).Error("failed to update meta event")
		}

		return nil
	}
}

func sendUrlRequest(project *datastore.Project, metaEvent *datastore.MetaEvent) (*net.Response, error) {
	cfg, err := config.Get()
	if err != nil {
		return nil, err
	}

	httpDuration, err := time.ParseDuration(convoy.HTTP_TIMEOUT)
	if err != nil {
		log.WithError(err).Error("failed to parse timeout duration")
		return nil, err
	}

	dispatch, err := net.NewDispatcher(httpDuration, cfg.Server.HTTP.HttpProxy)
	if err != nil {
		log.WithError(err).Error("error occurred while creating http client")
		return nil, err
	}

	sig := &signature.Signature{
		Payload: json.RawMessage(metaEvent.Metadata.Raw),
		Schemes: []signature.Scheme{
			{
				Secret:   []string{project.Config.MetaEvent.Secret},
				Hash:     "SHA256",
				Encoding: "hex",
			},
		},
	}

	header, err := sig.ComputeHeaderValue()
	if err != nil {
		log.WithError(err).Error("error occurred generating hmac")
		return nil, err
	}

	url := project.Config.MetaEvent.URL
	resp, err := dispatch.SendRequest(url, string(convoy.HttpPost), sig.Payload, "X-Convoy-Signature", header, int64(cfg.MaxResponseSize), httpheader.HTTPHeader{})
	if err != nil {
		return nil, err
	}

	var status string
	var statusCode int

	start := time.Now()
	if resp != nil {
		status = resp.Status
		statusCode = resp.StatusCode
	}
	requestLogger := log.WithFields(log.Fields{
		"status":   status,
		"uri":      url,
		"method":   convoy.HttpPost,
		"duration": time.Since(start),
	})

	if statusCode >= 200 && statusCode <= 299 {
		requestLogger.Infof("%s", metaEvent.UID)
		log.Infof("%s sent", metaEvent.UID)
		return resp, nil
	}

	requestLogger.Errorf("%s", metaEvent.UID)
	return resp, errors.New(resp.Error)
}
