package config

import (
	"os"
	"strconv"
	"testing"

	"github.com/frain-dev/convoy"
	"github.com/stretchr/testify/require"
)

func Test_EnvironmentTakesPrecedence(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		testType  string
		envConfig string
	}{
		{
			name:      "Port (number)",
			key:       "PORT",
			testType:  "number",
			envConfig: "8080",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup.
			os.Setenv(tc.key, tc.envConfig)
			defer os.Unsetenv(tc.key)

			configFile := "./testdata/Test_ConfigurationFromEnvironment/convoy.json"
			err := LoadConfig(configFile)
			require.NoError(t, err)

			cfg, _ := Get()

			// Assert.
			switch tc.testType {
			case "number":
				port, e := strconv.ParseInt(tc.envConfig, 10, 64)
				require.NoError(t, e)
				require.Equal(t, port, int64(cfg.Server.HTTP.Port))
			}
		})
	}
}

func Test_NilEnvironmentVariablesDontOverride(t *testing.T) {
	tests := []struct {
		name      string
		key       string
		testType  string
		envConfig string
		expected  string
	}{
		{
			name:      "Port (number)",
			key:       "PORT",
			testType:  "number",
			envConfig: "0",
			expected:  "8080",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup.
			configFile := "./testdata/Test_ConfigurationFromEnvironment/convoy.json"
			err := LoadConfig(configFile)

			require.NoError(t, err)

			cfg, _ := Get()

			// Assert.
			switch tc.testType {
			case "number":
				port, e := strconv.ParseInt(tc.expected, 10, 64)
				require.NoError(t, e)
				require.Equal(t, port, int64(cfg.Server.HTTP.Port))
			}
		})
	}
}

func TestLoadConfig(t *testing.T) {
	type args struct {
		path string
	}
	tests := []struct {
		name       string
		args       args
		wantCfg    Configuration
		wantErr    bool
		wantErrMsg string
	}{
		{
			name: "should_load_config_successfully",
			args: args{
				path: "./testdata/Config/valid-convoy.json",
			},
			wantCfg: Configuration{
				Host: "localhost:5005",
				Database: DatabaseConfiguration{
					Type:                  PostgresDatabaseProvider,
					Dsn:                   "postgres://inside-config-file",
					SetMaxOpenConnections: 10,
					SetMaxIdleConnections: 10,
					SetConnMaxLifetime:    3600,
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Dsn:  "redis://localhost:8379",
				},
				Search: DefaultConfiguration.Search,
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port:       80,
						WorkerPort: 5006,
					},
				},
				Logger: LoggerConfiguration{
					Level: "error",
				},
				MaxResponseSize: 40 * 1024,
				Environment:     OSSEnvironment,
				Auth: AuthConfiguration{
					IsSignupEnabled: true,
				},
				Analytics: AnalyticsConfiguration{
					IsEnabled: true,
				},
				StoragePolicy: StoragePolicyConfiguration{
					Type: "on-prem",
					OnPrem: OnPremStorage{
						Path: convoy.DefaultOnPremDir,
					},
				},
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_switch_to_default_MaxResponseSize_for_too_large_config",
			args: args{
				path: "./testdata/Config/too-large-max-response-size-convoy.json",
			},
			wantCfg: Configuration{
				Host: "localhost:5005",
				Database: DatabaseConfiguration{
					Type:                  PostgresDatabaseProvider,
					Dsn:                   "postgres://inside-config-file",
					SetMaxOpenConnections: 10,
					SetMaxIdleConnections: 10,
					SetConnMaxLifetime:    3600,
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Dsn:  "redis://localhost:8379",
				},
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port:       80,
						WorkerPort: 5006,
					},
				},
				Logger: LoggerConfiguration{
					Level: "error",
				},
				Search:          DefaultConfiguration.Search,
				MaxResponseSize: MaxResponseSize,
				Environment:     OSSEnvironment,
				Auth: AuthConfiguration{
					IsSignupEnabled: true,
				},
				Analytics: AnalyticsConfiguration{
					IsEnabled: true,
				},
				StoragePolicy: StoragePolicyConfiguration{
					Type: "on-prem",
					OnPrem: OnPremStorage{
						Path: convoy.DefaultOnPremDir,
					},
				},
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_switch_to_default_MaxResponseSize_for_zero_config",
			args: args{
				path: "./testdata/Config/zero-max-response-size-convoy.json",
			},
			wantCfg: Configuration{
				Host: "localhost:5005",
				Database: DatabaseConfiguration{
					Type:                  PostgresDatabaseProvider,
					Dsn:                   "postgres://inside-config-file",
					SetMaxOpenConnections: 10,
					SetMaxIdleConnections: 10,
					SetConnMaxLifetime:    3600,
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Dsn:  "redis://localhost:8379",
				},
				Search: DefaultConfiguration.Search,
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port:       80,
						WorkerPort: 5006,
					},
				},
				Logger: LoggerConfiguration{
					Level: "error",
				},
				MaxResponseSize: MaxResponseSize,
				Environment:     OSSEnvironment,
				Auth: AuthConfiguration{
					IsSignupEnabled: true,
				},
				Analytics: AnalyticsConfiguration{
					IsEnabled: true,
				},
				StoragePolicy: StoragePolicyConfiguration{
					Type: "on-prem",
					OnPrem: OnPremStorage{
						Path: convoy.DefaultOnPremDir,
					},
				},
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_error_for_empty_ssl_key_file",
			args: args{
				path: "./testdata/Config/empty-ssl-key-file.json",
			},
			wantCfg:    Configuration{},
			wantErr:    true,
			wantErrMsg: "both cert_file and key_file are required for ssl",
		},
		{
			name: "should_error_for_empty_ssl_cert_file",
			args: args{
				path: "./testdata/Config/empty-ssl-cert-file.json",
			},
			wantCfg:    Configuration{},
			wantErr:    true,
			wantErrMsg: "both cert_file and key_file are required for ssl",
		},
		{
			name: "should_error_for_empty_redis_dsn",
			args: args{
				path: "./testdata/Config/empty-redis-dsn.json",
			},
			wantErr:    true,
			wantErrMsg: "redis queue dsn is empty",
		},
		{
			name: "should_error_for_unsupported_queue_type",
			args: args{
				path: "./testdata/Config/unsupported-queue-type.json",
			},
			wantErr:    true,
			wantErrMsg: "unsupported queue type: abc",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := LoadConfig(tt.args.path)

			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}

			require.NoError(t, err)

			cfg, err := Get()
			require.NoError(t, err)
			require.Equal(t, tt.wantCfg, cfg)
		})
	}
}

func TestConfigDefaults(t *testing.T) {
	err := LoadConfig("./testdata/Config/empty-config")
	require.NoError(t, err)

	cfg, err := Get()
	require.NoError(t, err)

	expectedCFG := DefaultConfiguration
	expectedCFG.MaxResponseSize = MaxResponseSize
	require.Equal(t, expectedCFG, cfg)
}

func TestOverride(t *testing.T) {
	type args struct {
		path string
	}

	tests := []struct {
		name       string
		args       args
		config     *Configuration
		configType string
	}{
		{
			name: "should_override_database_configuration",
			args: args{
				path: "./testdata/Config/valid-convoy.json",
			},
			config: &Configuration{
				Database: DatabaseConfiguration{
					Type:                  PostgresDatabaseProvider,
					SetMaxOpenConnections: 10,
					SetMaxIdleConnections: 10,
					SetConnMaxLifetime:    3600,
					Dsn:                   "localhost",
				},
			},
			configType: "database",
		},
		{
			name: "should_override_queue_configuration",
			args: args{
				path: "./testdata/Config/valid-convoy.json",
			},
			config: &Configuration{
				Database: DatabaseConfiguration{
					Type:                  PostgresDatabaseProvider,
					SetMaxOpenConnections: 10,
					SetMaxIdleConnections: 10,
					SetConnMaxLifetime:    3600,
					Dsn:                   "localhost",
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Dsn:  "localhost:6379",
				},
			},
			configType: "queue",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Arrange.
			// Setup Global Config.
			err := LoadConfig(tc.args.path)
			require.NoError(t, err)

			// Act.
			err = Override(tc.config)
			require.NoError(t, err)

			// Assert.
			c, err := Get()
			require.Nil(t, err)

			switch tc.configType {
			case "database":
				require.Equal(t, c.Database, tc.config.Database)
			case "queue":
				require.Equal(t, c.Queue, tc.config.Queue)
			default:
			}
		})
	}
}
