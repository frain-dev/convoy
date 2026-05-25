package services

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	_ = os.Setenv("CONVOY_JWT_SECRET", "test-access-secret")
	_ = os.Setenv("CONVOY_JWT_REFRESH_SECRET", "test-refresh-secret")
	os.Exit(m.Run())
}
