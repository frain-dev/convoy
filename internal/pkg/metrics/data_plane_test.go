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

			// values
			assert.Equal(t, float64(1), testutil.ToFloat64(m.IngestTotal.WithLabelValues(tt.args.source.ProjectID,
				tt.args.source.UID)))
			assert.Equal(t, float64(0), testutil.ToFloat64(m.IngestTotal.WithLabelValues(tt.args.source.ProjectID,
				"xo")))
		})
	}
}
