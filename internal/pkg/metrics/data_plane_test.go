package metrics

import (
	"github.com/frain-dev/convoy/datastore"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/stretchr/testify/assert"
	"math/rand"
	"reflect"
	"strconv"
	"testing"
)

func TestGetDPInstance(t *testing.T) {
	tests := []struct {
		name string
		want *Metrics
	}{
		{
			name: "Get same DP metrics singleton instance",
			want: GetDPInstance(),
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := GetDPInstance(); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("GetDPInstance() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestMetrics_IncrementDeliveredTotal(t *testing.T) {
	type args struct {
		pUID string
		eUID string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Increment delivered total",
			args: args{
				pUID: "pUID-" + strconv.Itoa(rand.Int()),
				eUID: "eUID-" + strconv.Itoa(rand.Int()),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitMetrics()

			prometheus.MustRegister(m.EgressDeliveredTotal)

			m.IncrementEgressDeliveredTotal(tt.args.pUID, tt.args.eUID)

			// collected just one metric for both event & attempt
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestConsumedTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestErrorsTotal))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressTotal))
			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveryLatency))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsTotal))
			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressAttemptsDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptDeliveryLatency))

			// values
			assert.Equal(t, float64(1), testutil.ToFloat64(m.EgressDeliveredTotal.WithLabelValues(tt.args.pUID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.EgressDeliveredTotal.WithLabelValues(tt.args.eUID)))
			assert.Equal(t, float64(1), testutil.ToFloat64(m.EgressAttemptsDeliveredTotal.WithLabelValues(tt.args.pUID, tt.args.eUID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.EgressAttemptsDeliveredTotal.WithLabelValues(tt.args.pUID, "xo")))
		})
	}
}

func TestMetrics_IncrementErrorsTotal(t *testing.T) {
	type args struct {
		pUID string
		eUID string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Increment errors total",
			args: args{
				pUID: "pUID-" + strconv.Itoa(rand.Int()),
				eUID: "eUID-" + strconv.Itoa(rand.Int()),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitMetrics()

			prometheus.MustRegister(m.EgressErrorsTotal)

			m.IncrementEgressErrorsTotal(tt.args.pUID, tt.args.eUID)

			// collected just one metric for both event & attempt
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestConsumedTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestErrorsTotal))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveredTotal))
			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveryLatency))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsDeliveredTotal))
			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressAttemptErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptDeliveryLatency))

			// values
			assert.Equal(t, float64(1), testutil.ToFloat64(m.EgressErrorsTotal.WithLabelValues(tt.args.pUID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.EgressErrorsTotal.WithLabelValues(tt.args.eUID)))
			assert.Equal(t, float64(1), testutil.ToFloat64(m.EgressAttemptErrorsTotal.WithLabelValues(tt.args.pUID, tt.args.eUID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.EgressAttemptErrorsTotal.WithLabelValues(tt.args.pUID, "xo")))
		})
	}
}

func TestMetrics_IncrementIngestConsumedTotal(t *testing.T) {
	type args struct {
		source *datastore.Source
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Increment ingest consumed total",
			args: args{
				source: &datastore.Source{
					UID:       "source-" + strconv.Itoa(rand.Int()),
					ProjectID: "project-" + strconv.Itoa(rand.Int()),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitMetrics()

			prometheus.MustRegister(m.IngestConsumedTotal)

			m.IncrementIngestConsumedTotal(tt.args.source)

			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestTotal))
			assert.Equal(t, 1, testutil.CollectAndCount(m.IngestConsumedTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestErrorsTotal))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveryLatency))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptDeliveryLatency))

			// values
			assert.Equal(t, float64(1), testutil.ToFloat64(m.IngestConsumedTotal.
				WithLabelValues(tt.args.source.ProjectID, tt.args.source.UID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.IngestConsumedTotal.
				WithLabelValues(tt.args.source.ProjectID, "xo")))
		})
	}
}

func TestMetrics_IncrementIngestErrorsTotal(t *testing.T) {
	type args struct {
		source *datastore.Source
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Increment ingest errors total",
			args: args{
				source: &datastore.Source{
					UID:       "source-" + strconv.Itoa(rand.Int()),
					ProjectID: "project-" + strconv.Itoa(rand.Int()),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitMetrics()

			prometheus.MustRegister(m.IngestErrorsTotal)

			m.IncrementIngestErrorsTotal(tt.args.source)

			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestConsumedTotal))
			assert.Equal(t, 1, testutil.CollectAndCount(m.IngestErrorsTotal))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveryLatency))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptDeliveryLatency))

			// values
			assert.Equal(t, float64(1), testutil.ToFloat64(m.IngestErrorsTotal.WithLabelValues(tt.args.source.ProjectID,
				tt.args.source.UID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.IngestErrorsTotal.WithLabelValues(tt.args.source.ProjectID,
				"xo")))
		})
	}
}

func TestMetrics_IncrementIngestTotal(t *testing.T) {
	type args struct {
		source *datastore.Source
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Increment ingest received total",
			args: args{
				source: &datastore.Source{
					UID:       "source-" + strconv.Itoa(rand.Int()),
					ProjectID: "project-" + strconv.Itoa(rand.Int()),
				},
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitMetrics()

			prometheus.MustRegister(m.IngestTotal)

			m.IncrementIngestTotal(tt.args.source)

			// collected just one metric for both event & attempt
			assert.Equal(t, 1, testutil.CollectAndCount(m.IngestTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestConsumedTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestErrorsTotal))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveryLatency))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptDeliveryLatency))

			// values
			assert.Equal(t, float64(1), testutil.ToFloat64(m.IngestTotal.WithLabelValues(tt.args.source.ProjectID,
				tt.args.source.UID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.IngestTotal.WithLabelValues(tt.args.source.ProjectID,
				"xo")))
		})
	}
}

func TestMetrics_IncrementEgressTotal(t *testing.T) {
	type args struct {
		pUID string
		eUID string
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Increment egress total",
			args: args{
				pUID: "pUID-" + strconv.Itoa(rand.Int()),
				eUID: "eUID-" + strconv.Itoa(rand.Int()),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitMetrics()

			prometheus.MustRegister(m.EgressTotal)

			m.IncrementEgressTotal(tt.args.pUID, tt.args.eUID)

			// collected just one metric for both event & attempt
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestConsumedTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestErrorsTotal))

			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveryLatency))

			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressAttemptsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptDeliveryLatency))

			// values
			assert.Equal(t, float64(1), testutil.ToFloat64(m.EgressTotal.WithLabelValues(tt.args.pUID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.EgressTotal.WithLabelValues("xo")))
			assert.Equal(t, float64(1), testutil.ToFloat64(m.EgressAttemptsTotal.WithLabelValues(tt.args.pUID, tt.args.eUID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.EgressAttemptsTotal.WithLabelValues(tt.args.pUID, "xo")))

		})
	}
}

func TestMetrics_ObserveEgressDeliveryLatency(t *testing.T) {
	type args struct {
		pUID    string
		eUID    string
		elapsed int64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Observe delivery latency",
			args: args{
				pUID:    "pUID-" + strconv.Itoa(rand.Int()),
				eUID:    "eUID-" + strconv.Itoa(rand.Int()),
				elapsed: rand.Int63(),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitMetrics()

			prometheus.MustRegister(m.EgressDeliveryLatency, m.EgressAttemptDeliveryLatency)

			m.ObserveEgressDeliveryLatency(tt.args.pUID, tt.args.eUID, tt.args.elapsed)

			// collected just one metric for both event & attempt
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestConsumedTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestErrorsTotal))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressNetworkLatency))
			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressDeliveryLatency))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptErrorsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptNetworkLatency))
			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressAttemptDeliveryLatency))
		})
	}
}

func TestMetrics_ObserveEgressNetworkLatency(t *testing.T) {
	type args struct {
		pUID    string
		eUID    string
		elapsed int64
	}
	tests := []struct {
		name string
		args args
	}{
		{
			name: "Observe delivery network latency",
			args: args{
				pUID: "pUID-" + strconv.Itoa(rand.Int()),
				eUID: "eUID-" + strconv.Itoa(rand.Int()),
			},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := InitMetrics()

			prometheus.MustRegister(m.EgressNetworkLatency, m.EgressAttemptNetworkLatency)

			m.ObserveEgressNetworkLatency(tt.args.pUID, tt.args.eUID, tt.args.elapsed)

			// collected just one metric for both event & attempt
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestConsumedTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.IngestErrorsTotal))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressErrorsTotal))
			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressDeliveryLatency))

			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptsDeliveredTotal))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptErrorsTotal))
			assert.Equal(t, 1, testutil.CollectAndCount(m.EgressAttemptNetworkLatency))
			assert.Equal(t, 0, testutil.CollectAndCount(m.EgressAttemptDeliveryLatency))
		})
	}
}
