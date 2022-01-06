package config

import (
	"os"
	"testing"

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
			name:      "DB DSN - Takes priority",
			key:       "CONVOY_MONGO_DSN",
			testType:  "db",
			envConfig: "subomi",
		},
		{
			name:      "Queue DSN - Takes priority",
			key:       "CONVOY_REDIS_DSN",
			testType:  "queue",
			envConfig: "queue-set",
		},
		{
			name:      "Signature Header - Takes priority",
			key:       "CONVOY_SIGNATURE_HEADER",
			testType:  "header",
			envConfig: "X-Convoy-Test-Signature",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup.
			os.Setenv(tc.key, tc.envConfig)
			defer os.Unsetenv(tc.key)

			// Assert.
			configFile := "./testdata/Test_ConfigurationFromEnvironment/convoy.json"
			err := LoadConfig(configFile, new(Configuration))
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			cfg, _ := Get()

			errString := "Environment variable - %s didn't take precedence"
			switch tc.testType {
			case "queue":
				if cfg.Queue.Redis.DSN != tc.envConfig {
					t.Errorf(errString, tc.testType)
				}
			case "db":
				if cfg.Database.Dsn != tc.envConfig {
					t.Errorf(errString, tc.testType)
				}
			case "header":
				if string(cfg.GroupConfig.Signature.Header) != tc.envConfig {
					t.Errorf(errString, tc.testType)
				}
			}
		})
	}
}

func Test_CliFlagsTakePrecedenceOverConfigFile(t *testing.T) {
	tests := []struct {
		name      string
		testType  string
		flagValue string
	}{
		{
			name:      "DB DSN - Takes priority",
			testType:  "db",
			flagValue: "mongo://some-link",
		},
		{
			name:      "Queue DSN - Takes priority",
			testType:  "queue",
			flagValue: "redis://some-link",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup.
			ov := new(Configuration)

			switch tc.testType {
			case "queue":
				ov.Queue.Redis.DSN = tc.flagValue
			case "db":
				ov.Database.Dsn = tc.flagValue
			}

			// Assert.
			configFile := "./testdata/Config/valid-convoy.json"
			err := LoadConfig(configFile, ov)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			cfg, _ := Get()

			errString := "Cli Flag - %s didn't take precedence"
			switch tc.testType {
			case "queue":
				if cfg.Queue.Redis.DSN != tc.flagValue {
					t.Errorf(errString, tc.testType)
				}
			case "db":
				if cfg.Database.Dsn != tc.flagValue {
					t.Errorf(errString, tc.testType)
				}
			}
		})
	}
}

func Test_CliFlagsTakePrecedenceOverEnvironmentVariables(t *testing.T) {
	tests := []struct {
		name      string
		testType  string
		flagValue string
		key       string
		envConfig string
	}{
		{
			name:      "DB DSN - Takes priority",
			testType:  "db",
			flagValue: "mongo://some-link",
			key:       "CONVOY_MONGO_DSN",
			envConfig: "subomi",
		},
		{
			name:      "Queue DSN - Takes priority",
			testType:  "queue",
			flagValue: "redis://some-link",
			key:       "CONVOY_REDIS_DSN",
			envConfig: "queue-set",
		},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Setup.
			os.Setenv(tc.key, tc.envConfig)
			defer os.Unsetenv(tc.key)
			ov := new(Configuration)

			switch tc.testType {
			case "queue":
				ov.Queue.Redis.DSN = tc.flagValue
			case "db":
				ov.Database.Dsn = tc.flagValue
			}

			// Assert.
			configFile := "./testdata/Config/valid-convoy.json"
			err := LoadConfig(configFile, ov)
			if err != nil {
				t.Errorf("Failed to load config file: %v", err)
			}

			cfg, _ := Get()

			errString := "Cli Flag - %s didn't take precedence"
			switch tc.testType {
			case "queue":
				if cfg.Queue.Redis.DSN != tc.flagValue {
					t.Errorf(errString, tc.testType)
				}
			case "db":
				if cfg.Database.Dsn != tc.flagValue {
					t.Errorf(errString, tc.testType)
				}
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
				Database: DatabaseConfiguration{
					Dsn: "mongodb://inside-config-file",
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Redis: RedisQueueConfiguration{
						DSN: "redis://localhost:8379",
					},
				},
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port: 80,
					},
				},
				GroupConfig: GroupConfig{
					Strategy: StrategyConfiguration{
						Type: "default",
						Default: DefaultStrategyConfiguration{
							IntervalSeconds: 125,
							RetryLimit:      15,
						},
					},
					Signature: SignatureConfiguration{
						Header: DefaultSignatureHeader,
						Hash:   "SHA256",
					},
					DisableEndpoint: false,
				},
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
				Database: DatabaseConfiguration{
					Dsn: "mongodb://inside-config-file",
				},
				Queue: QueueConfiguration{
					Type: RedisQueueProvider,
					Redis: RedisQueueConfiguration{
						DSN: "redis://localhost:8379",
					},
				},
				Server: ServerConfiguration{
					HTTP: HTTPServerConfiguration{
						Port: 80,
					},
				},
				Auth: AuthConfiguration{
					RequireAuth: true,
					File: FileRealmOption{
						Basic: []BasicAuth{
							{
								Username: "123",
								Password: "abc",
								Role: Role{
									Type: "super_user",
								},
							},
						},
					},
				},
				GroupConfig: GroupConfig{
					Strategy: StrategyConfiguration{
						Type: "default",
						Default: DefaultStrategyConfiguration{
							IntervalSeconds: 125,
							RetryLimit:      15,
						},
					},
					Signature: SignatureConfiguration{
						Header: DefaultSignatureHeader,
						Hash:   "SHA256",
					},
					DisableEndpoint: false,
				},
				Environment:     DevelopmentEnvironment,
				MultipleTenants: false,
			},
			wantErr: false,
		},
		{
			name: "should_error_for_zero_port",
			args: args{
				path: "./testdata/Config/no-port-convoy.json",
			},
			wantCfg:    Configuration{},
			wantErr:    true,
			wantErrMsg: "http port cannot be zero",
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
			name: "should_error_for_invalid_signature_hash",
			args: args{
				path: "./testdata/Config/invalid-signature-hash.json",
			},
			wantCfg:    Configuration{},
			wantErr:    true,
			wantErrMsg: "invalid hash algorithm - 'SHA100', must be one of [MD5 SHA1 SHA224 SHA256 SHA384 SHA512 SHA3_224 SHA3_256 SHA3_384 SHA3_512 SHA512_224 SHA512_256]",
		},
		{
			name: "should_error_for_zero_interval_seconds",
			args: args{
				path: "./testdata/Config/zero-interval-seconds.json",
			},
			wantCfg:    Configuration{},
			wantErr:    true,
			wantErrMsg: "both interval seconds and retry limit are required for default strategy configuration",
		},
		{
			name: "should_error_for_zero_retry_limit",
			args: args{
				path: "./testdata/Config/zero-retry-limit.json",
			},
			wantCfg:    Configuration{},
			wantErr:    true,
			wantErrMsg: "both interval seconds and retry limit are required for default strategy configuration",
		},
		{
			name: "should_error_for_unsupported_strategy_type",
			args: args{
				path: "./testdata/Config/unknown-strategy-type.json",
			},
			wantErr:    true,
			wantErrMsg: "unsupported strategy type: abc",
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
			wantErrMsg: "please specify groups for basic auth",
		},
		{
			name: "should_error_for_empty_group",
			args: args{
				path: "./testdata/Config/empty-basic-auth-group-name.json",
			},
			wantErr:    true,
			wantErrMsg: "empty group name not allowed for basic auth",
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
			wantErrMsg: "empty group name not allowed for api-key auth",
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := LoadConfig(tt.args.path, new(Configuration))
			if tt.wantErr {
				require.NotNil(t, err)
				require.Equal(t, tt.wantErrMsg, err.Error())
				return
			}
			require.Nil(t, err)

			cfg, err := Get()
			require.Nil(t, err)

			require.Equal(t, tt.wantCfg, cfg)
		})
	}
}
