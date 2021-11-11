package config

import (
	"os"
	"testing"
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
			_ = LoadConfig(configFile)

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
