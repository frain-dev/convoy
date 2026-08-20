package postgres

import (
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/jmoiron/sqlx"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/stretchr/testify/require"

	"github.com/frain-dev/convoy/config"
	log "github.com/frain-dev/convoy/pkg/logger"
)

func resetPostgresCollectorState() {
	metricsMu.Lock()
	defer metricsMu.Unlock()
	cachedMetrics = nil
	lastRun = time.Time{}
	metricsInFlight = false
	metricsConfig = nil
}

func enableMetrics(t *testing.T) {
	t.Helper()
	t.Setenv("CONVOY_JWT_SECRET", "postgres-collector-test-secret")
	t.Setenv("CONVOY_JWT_REFRESH_SECRET", "postgres-collector-test-refresh")
	require.NoError(t, config.LoadConfig(""))
	require.NoError(t, config.Override(&config.Configuration{
		Metrics: config.MetricsConfiguration{
			IsEnabled: true,
			Backend:   config.PrometheusMetricsProvider,
			Prometheus: config.PrometheusMetricsConfiguration{
				SampleTime:   5,
				QueryTimeout: 30,
			},
		},
	}))
}

func delayedMetricsDB(t *testing.T) (*Postgres, sqlmock.Sqlmock) {
	t.Helper()
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	mock.ExpectQuery(`(?s).*`).WillDelayFor(2 * time.Second).WillReturnRows(
		sqlmock.NewRows([]string{"exists"}).AddRow(false),
	)

	p := &Postgres{
		dbx:    sqlx.NewDb(db, "sqlmock"),
		logger: log.New("postgres-collector-test", log.LevelError),
	}
	t.Cleanup(waitForMetricsRefresh)
	return p, mock
}

func waitForMetricsRefresh() {
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		metricsMu.Lock()
		flying := metricsInFlight
		metricsMu.Unlock()
		if !flying {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
}

func TestDescribeDoesNotQuery(t *testing.T) {
	resetPostgresCollectorState()
	enableMetrics(t)
	p, _ := delayedMetricsDB(t)

	ch := make(chan *prometheus.Desc, 16)
	start := time.Now()
	p.Describe(ch)
	require.Less(t, time.Since(start), 200*time.Millisecond)

	close(ch)
	var n int
	for range ch {
		n++
	}
	require.Equal(t, 5, n)
}

func TestCollectDoesNotQueryOnTheCaller(t *testing.T) {
	resetPostgresCollectorState()
	enableMetrics(t)
	p, _ := delayedMetricsDB(t)

	ch := make(chan prometheus.Metric, 16)
	start := time.Now()
	p.Collect(ch)
	require.Less(t, time.Since(start), 200*time.Millisecond)
}

func TestStartMetricsCollectionDoesNotQueryOnTheCaller(t *testing.T) {
	resetPostgresCollectorState()
	enableMetrics(t)
	p, _ := delayedMetricsDB(t)

	start := time.Now()
	p.StartMetricsCollection()
	require.Less(t, time.Since(start), 200*time.Millisecond)
}

func TestPrometheusRegisterDoesNotQueryOnTheCaller(t *testing.T) {
	resetPostgresCollectorState()
	enableMetrics(t)
	p, _ := delayedMetricsDB(t)

	start := time.Now()
	require.NoError(t, prometheus.NewPedanticRegistry().Register(p))
	require.Less(t, time.Since(start), 200*time.Millisecond)
}

func TestCollectRespectsSampleWindowAfterFailedRefresh(t *testing.T) {
	resetPostgresCollectorState()
	enableMetrics(t)
	lastRun = time.Now()

	p, _ := delayedMetricsDB(t)
	start := time.Now()
	p.Collect(make(chan prometheus.Metric, 1))
	require.Less(t, time.Since(start), 200*time.Millisecond)
}

func TestCollectIsNoopWhenMetricsDisabled(t *testing.T) {
	resetPostgresCollectorState()
	t.Setenv("CONVOY_JWT_SECRET", "postgres-collector-test-secret")
	t.Setenv("CONVOY_JWT_REFRESH_SECRET", "postgres-collector-test-refresh")
	require.NoError(t, config.LoadConfig(""))
	require.NoError(t, config.Override(&config.Configuration{
		Metrics: config.MetricsConfiguration{IsEnabled: false},
	}))

	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	t.Cleanup(func() { _ = db.Close() })

	p := &Postgres{dbx: sqlx.NewDb(db, "sqlmock"), logger: log.New("postgres-collector-test", log.LevelError)}
	p.Collect(make(chan prometheus.Metric, 1))
	require.NoError(t, mock.ExpectationsWereMet())
}
