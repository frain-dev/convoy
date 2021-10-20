package task

import (
	"context"
	"encoding/json"
	"errors"
	"time"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/net"
	"github.com/frain-dev/convoy/queue"
	"github.com/frain-dev/convoy/smtp"
	"github.com/frain-dev/convoy/util"
	"github.com/google/uuid"
	log "github.com/sirupsen/logrus"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

var ErrDeliveryAttemptFailed = errors.New("Error sending event")

type EndpointError struct {
	delay time.Duration
	Err   error
}

func (e *EndpointError) Error() string {
	return e.Err.Error()
}

func (e *EndpointError) Delay() time.Duration {
	return e.delay
}

func ProcessMessages(appRepo convoy.ApplicationRepository, msgRepo convoy.MessageRepository, orgRepo convoy.GroupRepository) func(*queue.Job) error {
	return func(job *queue.Job) error {
		Id := job.MsgID

		// Load message from DB and switch state to prevent concurrent processing.
		m, err := msgRepo.FindMessageByID(context.Background(), Id)

		if err != nil {
			log.WithError(err).Errorf("Failed to load message - %s", Id)
			return nil
		}

		switch m.Status {
		case convoy.ProcessingMessageStatus,
			convoy.SuccessMessageStatus:
			return nil
		}

		err = msgRepo.UpdateStatusOfMessages(context.Background(), []convoy.Message{*m}, convoy.ProcessingMessageStatus)
		if err != nil {
			log.WithError(err).Error("failed to update status of messages - ")
			return nil
		}

		var attempt convoy.MessageAttempt
		var secret = m.AppMetadata.Secret

		cfg, err := config.Get()
		if err != nil {
			return &EndpointError{Err: err}
		}

		dispatch := net.NewDispatcher()

		var done = true

		// It's an error state for the open core to have more than one endpoints.
		e := m.AppMetadata.Endpoints[0]

		if e.Sent {
			log.Debugf("endpoint %s already merged with message %s\n", e.TargetURL, m.UID)
			return nil
		}

		dbEndpoint, err := appRepo.FindApplicationEndpointByID(context.Background(), m.AppID, e.UID)
		if err != nil {
			log.WithError(err).Errorf("could not retrieve endpoint %s", e.UID)
			return &EndpointError{Err: err}
		}

		if dbEndpoint.Status == convoy.InactiveEndpointStatus {
			log.Debugf("endpoint %s is inactive, failing to send.", e.TargetURL)
			return nil
		}

		bytes, err := json.Marshal(m.Data)
		if err != nil {
			log.Errorf("error occurred while parsing json")
			return &EndpointError{Err: err}
		}

		bStr := string(bytes)
		hmac, err := util.ComputeJSONHmac(cfg.Signature.Hash, bStr, secret, false)
		if err != nil {
			log.Errorf("error occurred while generating hmac signature - %+v\n", err)
			return &EndpointError{Err: err}
		}

		attemptStatus := convoy.FailureMessageStatus
		start := time.Now()

		resp, err := dispatch.SendRequest(e.TargetURL, string(convoy.HttpPost), bytes, string(cfg.Signature.Header), hmac)
		status := "-"
		statusCode := 0
		if resp != nil {
			status = resp.Status
			statusCode = resp.StatusCode
		}

		duration := time.Since(start)
		// log request details
		requestLogger := log.WithFields(log.Fields{
			"status":   status,
			"uri":      e.TargetURL,
			"method":   convoy.HttpPost,
			"duration": duration,
		})

		if err == nil && statusCode >= 200 && statusCode <= 299 {
			requestLogger.Infof("%s", m.UID)
			log.Infof("%s sent", m.UID)
			attemptStatus = convoy.SuccessMessageStatus
			e.Sent = true

			m.Status = convoy.SuccessMessageStatus
			m.Description = ""
		} else {
			requestLogger.Errorf("%s", m.UID)
			done = false
			e.Sent = false

			m.Status = convoy.RetryMessageStatus

			delay := m.Metadata.IntervalSeconds
			nextTime := time.Now().Add(time.Duration(delay) * time.Second)
			m.Metadata.NextSendTime = primitive.NewDateTimeFromTime(nextTime)
			attempts := m.Metadata.NumTrials + 1

			log.Errorf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", m.UID, nextTime.Format(time.ANSIC), m.Metadata.Strategy, delay, attempts, m.Metadata.RetryLimit)
		}

		// Request failed but statusCode is 200 <= x <= 299
		if err != nil {
			log.Errorf("%s failed. Reason: %s", m.UID, err)
		}

		if done && dbEndpoint.Status == convoy.PendingEndpointStatus && cfg.DisableEndpoint {
			endpoints := []string{dbEndpoint.UID}
			endpointStatus := convoy.ActiveEndpointStatus

			err := appRepo.UpdateApplicationEndpointsStatus(context.Background(), m.AppID, endpoints, endpointStatus)
			if err != nil {
				log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
			}

			s, err := smtp.New(&cfg.SMTP)
			if err == nil {
				err = sendEmailNotification(m.AppMetadata, &orgRepo, s, endpointStatus)
				if err != nil {
					log.WithError(err).Error("Failed to send notification email")
				}
			}
		}

		if !done && dbEndpoint.Status == convoy.PendingEndpointStatus {
			endpoints := []string{dbEndpoint.UID}
			endpointStatus := convoy.InactiveEndpointStatus

			err := appRepo.UpdateApplicationEndpointsStatus(context.Background(), m.AppID, endpoints, endpointStatus)
			if err != nil {
				log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
			}
		}

		attempt = parseAttemptFromResponse(*m, e, resp, attemptStatus)

		m.Metadata.NumTrials++

		if m.Metadata.NumTrials >= m.Metadata.RetryLimit {
			if done {
				if m.Status != convoy.SuccessMessageStatus {
					log.Errorln("an anomaly has occurred. retry limit exceeded, fan out is done but event status is not successful")
					m.Status = convoy.FailureMessageStatus
				}
			} else {
				log.Errorf("%s retry limit exceeded ", m.UID)
				m.Description = "Retry limit exceeded"
				m.Status = convoy.FailureMessageStatus
			}

			if cfg.DisableEndpoint && dbEndpoint.Status != convoy.PendingEndpointStatus {
				endpoints := []string{dbEndpoint.UID}
				endpointStatus := convoy.InactiveEndpointStatus

				err := appRepo.UpdateApplicationEndpointsStatus(context.Background(), m.AppID, endpoints, endpointStatus)
				if err != nil {
					log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
				}

				s, err := smtp.New(&cfg.SMTP)
				if err == nil {
					err = sendEmailNotification(m.AppMetadata, &orgRepo, s, endpointStatus)
					if err != nil {
						log.WithError(err).Error("Failed to send notification email")
					}
				}
			}
		}

		err = msgRepo.UpdateMessageWithAttempt(context.Background(), *m, attempt)
		if err != nil {
			log.WithError(err).Error("failed to update message ", m.UID)
		}

		if !done && m.Metadata.NumTrials < m.Metadata.RetryLimit {
			delay := time.Duration(m.Metadata.IntervalSeconds) * time.Second
			return &EndpointError{Err: ErrDeliveryAttemptFailed, delay: delay}
		}

		return nil
	}
}

func sendEmailNotification(m *convoy.AppMetadata, o *convoy.GroupRepository, s *smtp.SmtpClient, status convoy.EndpointStatus) error {
	email := m.SupportEmail

	org, err := (*o).FetchGroupByID(context.Background(), m.GroupID)
	if err != nil {
		return err
	}

	logoURL := org.LogoURL

	for i := 0; i < len(m.Endpoints); i++ {
		endpoint := m.Endpoints[i]
		err = s.SendEmailNotification(email, logoURL, endpoint.TargetURL, status)
		if err != nil {
			return err
		}
	}

	return nil
}

func parseAttemptFromResponse(m convoy.Message, e convoy.EndpointMetadata, resp *net.Response, attemptStatus convoy.MessageStatus) convoy.MessageAttempt {

	responseHeader := util.ConvertDefaultHeaderToCustomHeader(&resp.ResponseHeader)
	requestHeader := util.ConvertDefaultHeaderToCustomHeader(&resp.RequestHeader)

	return convoy.MessageAttempt{
		ID:         primitive.NewObjectID(),
		UID:        uuid.New().String(),
		URL:        resp.URL.String(),
		Method:     resp.Method,
		MsgID:      m.UID,
		EndpointID: e.UID,
		APIVersion: "2021-08-27",

		IPAddress:        resp.IP,
		ResponseHeader:   *responseHeader,
		RequestHeader:    *requestHeader,
		HttpResponseCode: resp.Status,
		ResponseData:     string(resp.Body),
		Error:            resp.Error,
		Status:           attemptStatus,

		CreatedAt: primitive.NewDateTimeFromTime(time.Now()),
		UpdatedAt: primitive.NewDateTimeFromTime(time.Now()),
	}
}
