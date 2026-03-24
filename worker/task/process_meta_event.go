package task

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/hibiken/asynq"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/pkg/dedup"
	tracer2 "github.com/frain-dev/convoy/internal/pkg/tracer"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/pkg/httpheader"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/pkg/signature"
	"github.com/frain-dev/convoy/retrystrategies"
)

var ErrMetaEventDeliveryFailed = errors.New("meta event delivery failed")

type MetaEvent struct {
	MetaEventID string
	ProjectID   string
}

func ProcessMetaEvent(projectRepo datastore.ProjectRepository, metaEventRepo datastore.MetaEventRepository, dispatch *net.Dispatcher, tracerBackend tracer2.Backend) func(context.Context, *asynq.Task) error {
	return func(ctx context.Context, t *asynq.Task) error {
		var data MetaEvent

		err := msgpack.DecodeMsgPack(t.Payload(), &data)
		if err != nil {
			slog.Error("failed to unmarshal process process meta event payload", "error", err)
			err := json.Unmarshal(t.Payload(), &data)
			if err != nil {
				return &EndpointError{Err: err, delay: defaultDelay}
			}
		}

		metaEvent, err := metaEventRepo.FindMetaEventByID(ctx, data.ProjectID, data.MetaEventID)
		if err != nil {
			slog.Error("failed to find meta event by id", "error", err)
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		project, err := projectRepo.FetchProjectByID(ctx, data.ProjectID)
		if err != nil {
			slog.Error("failed to find project by id", "error", err)
			return &EndpointError{Err: err, delay: defaultDelay}
		}

		switch metaEvent.Status {
		case datastore.ProcessingEventStatus, datastore.SuccessEventStatus:
			return nil
		}

		metaEvent.Status = datastore.ProcessingEventStatus
		err = metaEventRepo.UpdateMetaEvent(ctx, metaEvent.ProjectID, metaEvent)
		if err != nil {
			slog.Error("failed to update meta event status", "error", err)
			return &EndpointError{Err: err, delay: defaultDelay}
		}
		cfg, err := config.Get()
		if err != nil {
			slog.Error("failed to get config", "error", err)
			return &EndpointError{Err: err, delay: defaultDelay}
		}
		metaEvent.Metadata.MaxRetrySeconds = cfg.MaxRetrySeconds

		delayDuration := retrystrategies.NewRetryStrategyFromMetadata(*metaEvent.Metadata).NextDuration(metaEvent.Metadata.NumTrials)

		resp, err := sendUrlRequest(ctx, project, metaEvent, dispatch, tracerBackend)
		metaEvent.Metadata.NumTrials++

		if resp != nil {
			responseHeader := datastore.ConvertDefaultHeaderToCustomHeader(&resp.ResponseHeader)
			requestHeader := datastore.ConvertDefaultHeaderToCustomHeader(&resp.RequestHeader)

			metaEvent.Attempt = &datastore.MetaEventAttempt{
				ResponseHeader: *responseHeader,
				RequestHeader:  *requestHeader,
				ResponseData:   string(resp.Body),
			}
		}

		if err != nil {
			slog.Error("failed to dispatch meta event request", "error", err)
			metaEvent.Status = datastore.RetryEventStatus
			nextTime := time.Now().Add(delayDuration)
			metaEvent.Metadata.NextSendTime = nextTime

			if metaEvent.Metadata.NumTrials >= metaEvent.Metadata.RetryLimit {
				metaEvent.Status = datastore.FailureEventStatus
			}

			err = metaEventRepo.UpdateMetaEvent(ctx, project.UID, metaEvent)
			if err != nil {
				slog.Error("failed to update meta event", "error", err)
			}

			if metaEvent.Metadata.NumTrials < metaEvent.Metadata.RetryLimit {
				slog.InfoContext(ctx, fmt.Sprintf("%s next retry time meta events is %s (strategy = %s, delay = %d, attempts = %d/%d)",
					metaEvent.UID, nextTime.Format(time.ANSIC), metaEvent.Metadata.Strategy, metaEvent.Metadata.IntervalSeconds, metaEvent.Metadata.NumTrials, metaEvent.Metadata.RetryLimit))
				return &EndpointError{Err: ErrMetaEventDeliveryFailed, delay: delayDuration}
			}

			return nil
		}

		metaEvent.Status = datastore.SuccessEventStatus
		err = metaEventRepo.UpdateMetaEvent(ctx, project.UID, metaEvent)
		if err != nil {
			slog.Error("failed to update meta event", "error", err)
		}

		return nil
	}
}

func sendUrlRequest(ctx context.Context, project *datastore.Project, metaEvent *datastore.MetaEvent, dispatch *net.Dispatcher, tracerBackend tracer2.Backend) (*net.Response, error) {
	cfg, err := config.Get()
	if err != nil {
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
		slog.Error("error occurred generating hmac", "error", err)
		return nil, err
	}

	url := project.Config.MetaEvent.URL

	httpDuration := convoy.HTTP_TIMEOUT_IN_DURATION
	start := time.Now()
	resp, err := dispatch.SendWebhook(ctx, url, sig.Payload, "X-Convoy-Signature", header, int64(cfg.MaxResponseSize), httpheader.HTTPHeader{}, dedup.GenerateChecksum(metaEvent.UID), httpDuration, "application/json")
	if err != nil {
		return nil, err
	}

	var status string
	var statusCode int
	var body []byte

	if resp != nil {
		status = resp.Status
		statusCode = resp.StatusCode
		body = resp.Body
	}
	duration := time.Since(start)
	logAttrs := []any{"status", status,
		"uri", url,
		"method", convoy.HttpPost,
		"duration", duration}

	if statusCode >= 200 && statusCode <= 299 {
		slog.Info(metaEvent.UID, logAttrs...)
		slog.Info(fmt.Sprintf("%s sent", metaEvent.UID))
		return resp, nil
	}

	attributes := map[string]interface{}{
		"project.id":           project.UID,
		"endpoint.url":         url,
		"response.status":      status,
		"response.status_code": statusCode,
		"response.size_bytes":  len(body),
		"meta_event.id":        metaEvent.UID,
	}

	startTime := time.Now().Add(-duration)
	endTime := time.Now()
	tracerBackend.Capture(ctx, "meta_event_delivery", attributes, startTime, endTime)

	slog.Error(metaEvent.UID, logAttrs...)
	return resp, errors.New(resp.Error)
}
