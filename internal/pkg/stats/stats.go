package stats

import (
	"bytes"
	"context"
	"encoding/json"
	"github.com/PuerkitoBio/rehttp"
	"github.com/frain-dev/convoy"
	"github.com/frain-dev/convoy/datastore"
	"github.com/frain-dev/convoy/pkg/log"
	"io"
	"net/http"
	"time"
)

type Stats struct {
	client   *http.Client
	edr      datastore.EventDeliveryRepository
	upstream string
}

// Entry represents an instance's hourly usage entry
//
// License is the license key of the instance
// Count is the number of event deliveries counted in the last hour
// Timestamp is the time the entry was generated
type Entry struct {
	License   string    `json:"license,omitempty"`
	Count     uint64    `json:"count,omitempty"`
	Timestamp time.Time `json:"timestamp,omitempty"`
}

func NewStats(upstream string, edr datastore.EventDeliveryRepository) *Stats {
	// will retry if the number of retries is
	// - less than 5
	// - the request times out
	timeoutRetryFn := rehttp.RetryAll(
		rehttp.RetryMaxRetries(5),
		rehttp.RetryTimeoutErr(),
	)

	// will retry if the number of retries is
	// - less than 5
	// - the error code matches
	httpStatusRetryFn := rehttp.RetryAll(
		rehttp.RetryMaxRetries(5),
		rehttp.RetryStatuses(400, 404, 500),
	)

	tr := rehttp.NewTransport(
		nil, // will use http.DefaultTransport
		rehttp.RetryAny(timeoutRetryFn, httpStatusRetryFn),
		rehttp.ConstDelay(time.Second), // wait 1s between retries
	)

	client := &http.Client{
		Transport: tr,
		Timeout:   10 * time.Second,
	}

	return &Stats{
		edr:      edr,
		client:   client,
		upstream: upstream,
	}
}

// Record sends a request to the statistics server, retries 5 times if an error occurs
func (s *Stats) Record(ctx context.Context, license string) error {
	count, err := s.edr.CountInstanceEventDeliveries(ctx)
	if err != nil {
		return err
	}

	entry := Entry{
		License:   license,
		Count:     count,
		Timestamp: time.Now(),
	}

	jsonData, err := json.Marshal(entry)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, "POST", s.upstream, bytes.NewBuffer(jsonData))
	if err != nil {
		log.WithError(err).Error("error occurred while creating request")
		return err
	}

	req.Header.Add("Content-Type", "application/json")
	req.Header.Add("User-Agent", defaultUserAgent())

	resp, err := s.client.Do(req)
	if err != nil {
		log.WithError(err).Error("error occurred while sending request")
		return err
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		log.WithError(err).Error("error occurred while reading response")
		return err
	}

	log.Infof("stats server response: %+v", body)

	return nil
}

func defaultUserAgent() string {
	return "Convoy/" + convoy.GetVersion()
}
