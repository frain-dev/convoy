//go:build integration
// +build integration

package testcon

import (
	"context"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	"testing"
	"time"
)

type IntegrationTestSuite struct {
	suite.Suite
	*TestData
}

func (i *IntegrationTestSuite) SetupSuite() {
	t := i.T()
	identifier := tc.StackIdentifier("convoy_docker_test")
	compose, err := tc.NewDockerComposeWith(tc.WithStackFiles("./testdata/docker-compose-test.yml"), identifier)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal), "compose.Down()")
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// ignore ryuk error
	_ = compose.WaitForService("postgres", wait.NewLogStrategy("ready").WithStartupTimeout(60*time.Second)).
		WaitForService("redis_server", wait.NewLogStrategy("Ready to accept connections").WithStartupTimeout(10*time.Second)).
		WaitForService("migrate", wait.NewLogStrategy("migration up succeeded").WithStartupTimeout(60*time.Second)).
		Up(ctx, tc.Wait(true), tc.WithRecreate(api.RecreateNever))

	i.TestData = seedTestData(t)
}

func (i *IntegrationTestSuite) SetupTest() {

}

func (i *IntegrationTestSuite) TearDownTest() {

}

func TestIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(IntegrationTestSuite))
}
