package config

import (
	"os"
	"strconv"
	"testing"

	"github.com/frain-dev/convoy/auth"

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
		{
			name:      "Basic Auth (interface)",
			key:       "CONVOY_BASIC_AUTH_CONFIG",
			testType:  "interface",
			envConfig: "[{\"username\": \"some-admin\",\"password\": \"some-password\",\"role\": {\"type\": \"super_user\",\"group\": \"\"}}]",
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
			case "interface":
				basicAuth := BasicAuthConfig{}
				e := basicAuth.Decode(tc.envConfig)
				require.NoError(t, e)
				require.Equal(t, basicAuth, cfg.Auth.File.Basic)
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
		{
			name:      "Basic Auth (interface)",
			key:       "CONVOY_BASIC_AUTH_CONFIG",
			testType:  "interface",
			envConfig: "",
			expected:  "[{\"username\": \"admin\",\"password\": \"qwerty\",\"role\": {\"type\": \"super_user\",\"groups\": \"\"}}]",
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
			case "interface":
				basicAuth := BasicAuthConfig{}
				e := basicAuth.Decode(tc.expected)
				require.NoError(t, e)
				require.Equal(t, basicAuth, cfg.Auth.File.Basic)
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
					Type: MongodbDatabaseProvider,
					Dsn:  "mongodb://inside-config-file",
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Redis: RedisQueueConfiguration{
						Dsn: "redis://localhost:8379",
					},
				},
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port:       80,
						WorkerPort: 5006,
					},
				},
				MaxResponseSize: 40 * 1024,
				Environment:     DevelopmentEnvironment,
				MultipleTenants: false,
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
					Type: MongodbDatabaseProvider,
					Dsn:  "mongodb://inside-config-file",
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Redis: RedisQueueConfiguration{
						Dsn: "redis://localhost:8379",
					},
				},
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port:       80,
						WorkerPort: 5006,
					},
				},
				MaxResponseSize: MaxResponseSize,
				Environment:     DevelopmentEnvironment,
				MultipleTenants: false,
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
					Type: MongodbDatabaseProvider,
					Dsn:  "mongodb://inside-config-file",
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Redis: RedisQueueConfiguration{
						Dsn: "redis://localhost:8379",
					},
				},
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port:       80,
						WorkerPort: 5006,
					},
				},
				MaxResponseSize: MaxResponseSize,
				Environment:     DevelopmentEnvironment,
				MultipleTenants: false,
			},
			wantErr:    false,
			wantErrMsg: "",
		},
		{
			name: "should_allow_zero_groups_for_superuser",
			args: args{
				path: "./testdata/Config/zero-groups-for-superuser.json",
			},
			wantCfg: Configuration{
				Host: "localhost:5005",
				Database: DatabaseConfiguration{
					Type: MongodbDatabaseProvider,
					Dsn:  "mongodb://inside-config-file",
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Redis: RedisQueueConfiguration{
						Dsn: "redis://localhost:8379",
					},
				},
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port:       80,
						WorkerPort: 5006,
					},
				},
				MaxResponseSize: MaxResponseSize,
				Auth: AuthConfiguration{
					File: FileRealmOption{
						Basic: []BasicAuth{
							{
								Username: "123",
								Password: "abc",
								Role: auth.Role{
									Type: "super_user",
								},
							},
						},
					},
				},
				Environment:     DevelopmentEnvironment,
				MultipleTenants: false,
			},
			wantErr: false,
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
		{
			name: "should_error_for_empty_password_for_basic_auth",
			args: args{
				path: "./testdata/Config/empty-password-for-basic-auth.json",
			},
			wantErr:    true,
			wantErrMsg: "username and password are required for basic auth config",
		},
		{
			name: "should_error_for_invalid_role",
			args: args{
				path: "./testdata/Config/invalid-role.json",
			},
			wantErr:    true,
			wantErrMsg: "invalid role type: abc",
		},
		{
			name: "should_error_for_zero_groups",
			args: args{
				path: "./testdata/Config/zero-groups-for-non-superuser.json",
			},
			wantErr:    true,
			wantErrMsg: "please specify group for basic auth",
		},
		{
			name: "should_error_for_empty_group",
			args: args{
				path: "./testdata/Config/empty-basic-auth-group-name.json",
			},
			wantErr:    true,
			wantErrMsg: "please specify group for basic auth",
		},
		{
			name: "should_error_for_empty_api_key",
			args: args{
				path: "./testdata/Config/empty-api-key.json",
			},
			wantErr:    true,
			wantErrMsg: "api-key is required for api-key auth config",
		},

		{
			name: "should_error_for_empty_api_key_group_name",
			args: args{
				path: "./testdata/Config/empty-api-key-auth-group-name.json",
			},
			wantErr:    true,
			wantErrMsg: "please specify group for api-key auth",
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
	require.Equal(t, DefaultConfiguration, cfg)
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
					Type: MongodbDatabaseProvider,
					Dsn:  "localhost",
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
					Type: MongodbDatabaseProvider,
					Dsn:  "localhost",
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Redis: RedisQueueConfiguration{
						Dsn: "localhost:6379",
					},
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
