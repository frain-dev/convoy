package worker

import (
	"fmt"

	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/internal/email"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/frain-dev/convoy/pkg/msgpack"
	"github.com/frain-dev/convoy/queue"
)

// EnqueueCircuitBreakerEmails enqueues notification emails to endpoint support email and project owner.
// ownerEmail may be empty if unavailable. It is safe to call this with missing emails; those are skipped.
func EnqueueCircuitBreakerEmails(q queue.Queuer, lo *log.Logger, project *datastore.Project, endpoint *datastore.Endpoint, ownerEmail string) error {
	// Endpoint support email
	if endpoint != nil && endpoint.SupportEmail != "" {
		emailMsg := &email.Message{
			Email:        endpoint.SupportEmail,
			Subject:      "Endpoint Disabled - Circuit Breaker Triggered",
			TemplateName: email.TemplateEndpointUpdate,
			Params: map[string]string{
				"name":            endpoint.Name,
				"logo_url":        project.LogoURL,
				"target_url":      endpoint.Url,
				"failure_msg":     "Circuit breaker threshold exceeded",
				"response_body":   "",
				"failure_rate":    fmt.Sprintf("%.2f", endpoint.FailureRate),
				"status_code":     "0",
				"endpoint_status": "inactive",
			},
		}
		if err := enqueueEmail(q, emailMsg); err != nil {
			lo.WithError(err).Error("Failed to queue circuit breaker notification email")
		}
	}

	// Owner email
	if ownerEmail != "" {
		nameParam := project.Name
		targetURL := ""
		failureRate := ""
		if endpoint != nil {
			nameParam = fmt.Sprintf("%s (%s)", endpoint.Name, project.Name)
			targetURL = endpoint.Url
			failureRate = fmt.Sprintf("%.2f", endpoint.FailureRate)
		}

		emailMsg := &email.Message{
			Email:        ownerEmail,
			Subject:      "Project Endpoint Disabled - Circuit Breaker Triggered",
			TemplateName: email.TemplateEndpointUpdate,
			Params: map[string]string{
				"name":            nameParam,
				"logo_url":        project.LogoURL,
				"target_url":      targetURL,
				"failure_msg":     "Circuit breaker threshold exceeded",
				"response_body":   "",
				"failure_rate":    failureRate,
				"status_code":     "0",
				"endpoint_status": "inactive",
			},
		}
		if err := enqueueEmail(q, emailMsg); err != nil {
			lo.WithError(err).Error("Failed to queue circuit breaker notification email to owner")
		}
	}
	return nil
}

func enqueueEmail(q queue.Queuer, emailMsg *email.Message) error {
	bytes, err := msgpack.EncodeMsgPack(emailMsg)
	if err != nil {
		return err
	}
	job := &queue.Job{Payload: bytes, Delay: 0}
	return q.Write(convoy.NotificationProcessor, convoy.DefaultQueue, job)
}
