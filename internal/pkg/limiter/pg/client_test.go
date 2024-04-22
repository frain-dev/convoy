package pg

import (
	"context"
	"github.com/frain-dev/convoy/config"
	"github.com/frain-dev/convoy/database/postgres"
	"github.com/frain-dev/convoy/pkg/log"
	"github.com/stretchr/testify/require"
	"os"
	"testing"
)

func getConfig() config.Configuration {
	_ = os.Setenv("CONVOY_DB_HOST", os.Getenv("TEST_REDIS_HOST"))
	_ = os.Setenv("CONVOY_REDIS_SCHEME", os.Getenv("TEST_REDIS_SCHEME"))
	_ = os.Setenv("CONVOY_REDIS_PORT", os.Getenv("TEST_REDIS_PORT"))

	_ = os.Setenv("CONVOY_DB_HOST", os.Getenv("TEST_DB_HOST"))
	_ = os.Setenv("CONVOY_DB_SCHEME", os.Getenv("TEST_DB_SCHEME"))
	_ = os.Setenv("CONVOY_DB_USERNAME", os.Getenv("TEST_DB_USERNAME"))
	_ = os.Setenv("CONVOY_DB_PASSWORD", os.Getenv("TEST_DB_PASSWORD"))
	_ = os.Setenv("CONVOY_DB_DATABASE", os.Getenv("TEST_DB_DATABASE"))
	_ = os.Setenv("CONVOY_DB_PORT", os.Getenv("TEST_DB_PORT"))

	err := config.LoadConfig("")
	if err != nil {
		log.Fatal(err)
	}

	cfg, err := config.Get()
	if err != nil {
		log.Fatal(err)
	}

	return cfg
}

func BenchmarkTakeToken_TakeOneToken(b *testing.B) {
	db, err := postgres.NewDB(getConfig())
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rateLimiter := NewRateLimiter(db)
		tokenErr := rateLimiter.takeToken(context.Background(), "test", 100_000_000, 1)
		require.NoError(b, tokenErr)
	}
}

func BenchmarkTakeToken_TakeNone(b *testing.B) {
	db, err := postgres.NewDB(getConfig())
	require.NoError(b, err)

	b.ResetTimer()
	b.ReportAllocs()

	for i := 0; i < b.N; i++ {
		rateLimiter := NewRateLimiter(db)
		tokenErr := rateLimiter.takeToken(context.Background(), "test-2", 0, 0)
		require.NoError(b, tokenErr)
	}
}
