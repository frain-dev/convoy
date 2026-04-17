package rqm

import (
	"context"
	"fmt"
	"net/url"
	"testing"
	"time"

	amqp091 "github.com/rabbitmq/amqp091-go"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"

	"github.com/frain-dev/convoy/datastore"
)

func TestDialerConnectionString(t *testing.T) {
	tests := []struct {
		name    string
		cfg     *datastore.AmqpPubSubConfig
		wantURL string
		wantErr bool
	}{
		{
			name: "plain credentials",
			cfg: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   "localhost",
				Port:   "5672",
				Vhost:  strPtr("/"),
				Auth: &datastore.AmqpCredentials{
					User:     "guest",
					Password: "guest",
				},
			},
			wantURL: "amqp://guest:guest@localhost:5672//?heartbeat=30",
		},
		{
			name: "password with @ symbol",
			cfg: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   "localhost",
				Port:   "5672",
				Vhost:  strPtr("/"),
				Auth: &datastore.AmqpCredentials{
					User:     "user",
					Password: "p@ssword",
				},
			},
			wantURL: "amqp://user:p%40ssword@localhost:5672//?heartbeat=30",
		},
		{
			name: "password with # and ! symbols",
			cfg: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   "localhost",
				Port:   "5672",
				Vhost:  strPtr("/"),
				Auth: &datastore.AmqpCredentials{
					User:     "user",
					Password: "p@ss#word!",
				},
			},
			wantURL: "amqp://user:p%40ss%23word%21@localhost:5672//?heartbeat=30",
		},
		{
			name: "password with percent encoding",
			cfg: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   "localhost",
				Port:   "5672",
				Vhost:  strPtr("/"),
				Auth: &datastore.AmqpCredentials{
					User:     "user",
					Password: "pass%word",
				},
			},
			wantURL: "amqp://user:pass%25word@localhost:5672//?heartbeat=30",
		},
		{
			name: "password with colon and slash",
			cfg: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   "localhost",
				Port:   "5672",
				Vhost:  strPtr("/"),
				Auth: &datastore.AmqpCredentials{
					User:     "user",
					Password: "p:ass/word",
				},
			},
			wantURL: "amqp://user:p%3Aass%2Fword@localhost:5672//?heartbeat=30",
		},
		{
			name: "username with special characters",
			cfg: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   "localhost",
				Port:   "5672",
				Vhost:  strPtr("/"),
				Auth: &datastore.AmqpCredentials{
					User:     "user@domain",
					Password: "password",
				},
			},
			wantURL: "amqp://user%40domain:password@localhost:5672//?heartbeat=30",
		},
		{
			name: "no authentication",
			cfg: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   "localhost",
				Port:   "5672",
				Vhost:  strPtr("/"),
				Auth:   nil,
			},
			wantURL: "amqp://localhost:5672//?heartbeat=30",
		},
		{
			name: "custom vhost",
			cfg: &datastore.AmqpPubSubConfig{
				Schema: "amqp",
				Host:   "localhost",
				Port:   "5672",
				Vhost:  strPtr("myvhost"),
				Auth: &datastore.AmqpCredentials{
					User:     "guest",
					Password: "p@ss",
				},
			},
			wantURL: "amqp://guest:p%40ss@localhost:5672/myvhost?heartbeat=30",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			connString := buildConnectionString(tt.cfg)
			require.Equal(t, tt.wantURL, connString)

			// Verify the URL is parseable
			parsed, err := url.Parse(connString)
			require.NoError(t, err)

			// Verify credentials round-trip correctly
			if tt.cfg.Auth != nil {
				require.Equal(t, tt.cfg.Auth.User, parsed.User.Username())
				password, ok := parsed.User.Password()
				require.True(t, ok)
				require.Equal(t, tt.cfg.Auth.Password, password)
			}
		})
	}
}

func TestDialerWithEscapedPassword_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test in short mode")
	}

	ctx := context.Background()

	// Start RabbitMQ container with default guest/guest credentials
	req := testcontainers.ContainerRequest{
		Image:        "rabbitmq:3.12-management-alpine",
		ExposedPorts: []string{"5672/tcp", "15672/tcp"},
		Env: map[string]string{
			"RABBITMQ_DEFAULT_USER": "guest",
			"RABBITMQ_DEFAULT_PASS": "guest",
		},
		WaitingFor: wait.ForLog("Server startup complete").
			WithStartupTimeout(60 * time.Second),
	}

	container, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	require.NoError(t, err)
	t.Cleanup(func() { container.Terminate(ctx) })

	host, err := container.Host(ctx)
	require.NoError(t, err)
	mappedPort, err := container.MappedPort(ctx, "5672")
	require.NoError(t, err)

	// Create a user with special characters in the password via rabbitmqctl
	specialPassword := "p@ss#word!"
	specialUser := "testuser"
	for _, cmd := range [][]string{
		{"rabbitmqctl", "add_user", specialUser, specialPassword},
		{"rabbitmqctl", "set_permissions", "-p", "/", specialUser, ".*", ".*", ".*"},
	} {
		code, _, err := container.Exec(ctx, cmd)
		require.NoError(t, err)
		require.Equal(t, 0, code, "rabbitmqctl command failed: %v", cmd)
	}

	port := fmt.Sprintf("%d", mappedPort.Int())
	vhost := "/"

	// Test 1: Verify connection works with the escaped password using our buildConnectionString
	cfg := &datastore.AmqpPubSubConfig{
		Schema: "amqp",
		Host:   host,
		Port:   port,
		Queue:  "test-queue",
		Vhost:  &vhost,
		Auth: &datastore.AmqpCredentials{
			User:     specialUser,
			Password: specialPassword,
		},
	}

	connString := buildConnectionString(cfg)
	conn, err := amqp091.Dial(connString)
	require.NoError(t, err, "should connect with escaped password")
	defer conn.Close()

	ch, err := conn.Channel()
	require.NoError(t, err, "should open channel")
	defer ch.Close()

	// Declare and publish to a queue to confirm full functionality
	q, err := ch.QueueDeclare("test-queue", true, false, false, false, nil)
	require.NoError(t, err)

	err = ch.Publish("", q.Name, false, false, amqp091.Publishing{
		ContentType: "application/json",
		Body:        []byte(`{"test": "message"}`),
	})
	require.NoError(t, err, "should publish message with escaped password credentials")
}

func strPtr(s string) *string {
	return &s
}
