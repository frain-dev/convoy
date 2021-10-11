package task

import (
	"context"
	"encoding/json"
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

func ProcessMessages(appRepo convoy.ApplicationRepository, msgRepo convoy.MessageRepository) func(*queue.Job) EndpointError {
	return func(job *queue.Job) EndpointError {
		m := job.Data
		var attempt convoy.MessageAttempt
		var secret = m.AppMetadata.Secret

		cfg, err := config.Get()
		if err != nil {
			return EndpointError{Err: err}
		}

		dispatch := net.NewDispatcher()

		var done = true
		for _, e := range m.AppMetadata.Endpoints {

			if e.Sent {
				log.Debugf("endpoint %s already merged with message %s\n", e.TargetURL, m.UID)
				continue
			}

			dbEndpoint, err := appRepo.FindApplicationEndpointByID(context.Background(), m.AppID, e.UID)
			if err != nil {
				log.WithError(err).Errorf("could not retrieve endpoint %s", e.UID)
				continue
			}

			if dbEndpoint.Status == convoy.InactiveEndpointStatus {
				log.Debugf("endpoint %s is inactive, failing to send.", e.TargetURL)
				done = false
				continue
			}

			bytes, err := json.Marshal(m.Data)
			if err != nil {
				log.Errorf("error occurred while parsing json")
				return EndpointError{Err: err}
			}

			bStr := string(bytes)
			hmac, err := util.ComputeJSONHmac(cfg.Signature.Hash, bStr, secret, false)
			if err != nil {
				log.Errorf("error occurred while generating hmac signature - %+v\n", err)
				return EndpointError{Err: err}
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
				log.Infof("%s sent\n", m.UID)
				attemptStatus = convoy.SuccessMessageStatus
				e.Sent = true

				if dbEndpoint.Status == convoy.PendingEndpointStatus {
					dbEndpoint.Status = convoy.ActiveEndpointStatus

					activeEnpoints := []string{dbEndpoint.UID}
					err := appRepo.UpdateApplicationEndpointsStatus(context.Background(), m.AppID, activeEnpoints, convoy.ActiveEndpointStatus)
					if err != nil {
						log.WithError(err).Error("Failed to reactivate endpoint after successful retry")
					}
				}
			} else {
				requestLogger.Errorf("%s", m.UID)
				done = false
				e.Sent = false
			}
			if err != nil {
				log.Errorf("%s failed. Reason: %s", m.UID, err)
			}

			attempt = parseAttemptFromResponse(*m, e, resp, attemptStatus)
		}

		m.Metadata.NumTrials++

		if done {
			m.Status = convoy.SuccessMessageStatus
			m.Description = ""

		} else {
			m.Status = convoy.RetryMessageStatus

			delay := m.Metadata.IntervalSeconds
			nextTime := time.Now().Add(time.Duration(delay) * time.Second)
			m.Metadata.NextSendTime = primitive.NewDateTimeFromTime(nextTime)

			log.Errorf("%s next retry time is %s (strategy = %s, delay = %d, attempts = %d/%d)\n", m.UID, nextTime.Format(time.ANSIC), m.Metadata.Strategy, delay, m.Metadata.NumTrials, m.Metadata.RetryLimit)
		}

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

			go func() {
				inactiveEndpoints := make([]string, 0)
				for i := 0; i < len(m.AppMetadata.Endpoints); i++ {
					endpoint := m.AppMetadata.Endpoints[i]
					if !endpoint.Sent {
						inactiveEndpoints = append(inactiveEndpoints, endpoint.UID)
					}
				}
				err := appRepo.UpdateApplicationEndpointsStatus(context.Background(), m.AppID, inactiveEndpoints, convoy.InactiveEndpointStatus)
				if err != nil {
					log.WithError(err).Error("Failed to update disabled app endpoints")
					return
				}

				s, err := smtp.New(&cfg.SMTP)
				if err == nil {
					for i := 0; i < len(m.AppMetadata.Endpoints); i++ {
						email := m.AppMetadata.SupportEmail
						endpoint := m.AppMetadata.Endpoints[i]
						err = s.SendEmailNotification(email, endpoint)
						if err != nil {
							log.WithError(err).Error("Failed to send notification email")
						}
					}
				}
			}()
		}

		err = msgRepo.UpdateMessageWithAttempt(context.Background(), *m, attempt)
		if err != nil {
			log.Errorln("failed to update message ", m.UID)
		}

		return EndpointError{}
	}
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
