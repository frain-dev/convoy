//go:build docker_testcon
// +build docker_testcon

package testcon

import (
	"context"
	"github.com/docker/compose/v2/pkg/api"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	tc "github.com/testcontainers/testcontainers-go/modules/compose"
	"github.com/testcontainers/testcontainers-go/wait"
	"strings"
	"testing"
	"time"
)

type DockerE2EIntegrationTestSuite struct {
	suite.Suite
	*TestData
}

func (d *DockerE2EIntegrationTestSuite) SetupSuite() {
	t := d.T()
	identifier := tc.StackIdentifier("convoy_docker_test")
	compose, err := tc.NewDockerComposeWith(tc.WithStackFiles("./testdata/docker-compose-test.yml"), identifier)
	require.NoError(t, err)

	t.Cleanup(func() {
		require.NoError(t, compose.Down(context.Background(), tc.RemoveOrphans(true), tc.RemoveImagesLocal), "compose.Down()")
	})

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)

	// ignore ryuk error
	err = compose.WaitForService("postgres", wait.NewLogStrategy("ready").WithStartupTimeout(60*time.Second)).
		WaitForService("redis_server", wait.NewLogStrategy("Ready to accept connections").WithStartupTimeout(10*time.Second)).
		WaitForService("migrate", wait.NewLogStrategy("migration up succeeded").WithStartupTimeout(60*time.Second)).
		Up(ctx, tc.Wait(true), tc.WithRecreate(api.RecreateNever))
	if err != nil && !strings.Contains(err.Error(), "Ryuk") {
		require.NoError(t, err)
	}

	d.TestData = seedTestData(t)
}

func (d *DockerE2EIntegrationTestSuite) SetupTest() {

}

func (d *DockerE2EIntegrationTestSuite) TearDownTest() {

}

func TestDockerE2EIntegrationTestSuite(t *testing.T) {
	suite.Run(t, new(DockerE2EIntegrationTestSuite))
}
