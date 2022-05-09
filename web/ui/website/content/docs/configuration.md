---
title: Configuration
description: 'Convoy Configuration'
id: configuration
order: 4
---

# Configuration

You can configure Convoy by using one of or a combination of the methods below:
- creating a `config json file` (default)
- setting `environment variables`.
- setting `cli flags`

The order of preference when all the are used is `cli flags` > `environment variables` > `config json file`. Values set in the cli flags will override the same config value set with either env vars of in the config file.

#### Creating a config json file
An example configuration is shown below:

```json[Sample Config]
{
  "environment": "development",
  "base_url": "localhost:5005",
  "multiple_tenants": false,
  "database": {
    "type": "mongodb",
    "dsn": "mongodb://localhost:27017/convoy"
  },
  "queue": {
    "type": "redis",
    "redis": {
      "dsn": "redis://localhost:6379"
    }
  },
  "cache": {
    "type": "redis",
    "redis": {
      "dsn": "redis://localhost:6379"
    }
  },
  "limiter": {
    "type": "redis",
    "redis": {
      "dsn": "redis://localhost:6379"
    }
  },
  "server": {
    "http": {
      "ssl": false,
      "ssl_cert_file": "",
      "ssl_key_file": "",
      "port": 5005,
      "worker_port": 5006
    }
  },
  "group": {
    "signature": {
      "header": "X-Convoy-Signature",
      "hash": "SHA512"
    },
    "strategy": {
      "type": "default",
      "default": {
        "intervalSeconds": 20,
        "retryLimit": 3
      }
    }
  },
  "tracer": {
    "type": "new_relic"
  },
  "new_relic": {
    "license_key": "012345678909876543210",
    "app_name": "convoy",
    "config_enabled": true,
    "distributed_tracer_enabled": true
  },
  "disable_endpoint": false,
  "auth": {
    "require_auth": false,
    "native": {
      "enabled": true
    },
    "file": {
      "basic": [
        {
          "username": "admin",
          "password": "password",
          "role": {
            "type": "super_user",
            "groups": []
          }
        }
      ],
      "api-key": [
        {
          "api_key": "ABC1234",
          "role": {
            "type": "admin",
            "groups": ["group-uid-1", "group-uid-2"],
            "apps": ["apps-uid-1", "apps-uid-2"]
          }
        }
      ]
    }
  }
}
```

#### Parameters

-   `environment`: Configure which environment configure is running on. Defaults `development`.
-   `database`: Configures the main data store. Currently supported databases: `mongodb` and `in-memory` using [badgerdb](https://github.com/dgraph-io/badger), planned: `postgres`.
	```json[sample]
	{
	  "database": {
	    "type": "mongodb",
	    "dsn": "mongodb://localhost:27017/convoy"
	  },
	}
	```
-   `queue`, `cache` and `limiter`: This configures a queuing backend to use. Currently supported queuing, caching and rate limiter backends: `redis` and `in-memory`, planned queuing backends: `rabbitmq` and `sqs`.
	```json[sample]
	{
	   "queue": {
		   "type": "redis",
		   "redis": {
		     "dsn": "redis://localhost:6379"
		   }
	   }
	}
	```
-   `port`: Specifies which port Convoy should run on.
-   `worker_port`: Specifies which port Convoy workers should run on.
-   `auth`: This specifies authentication mechanism used to access Convoy's API. If `require_auth` is set to `false`, Convoy's API won't need to be authenticated. Convoy supports two authentication mechanisms:
	- `basic`: username and password
	- `api_key`: revocable API keys

	```json[sample]
	{
	  "auth": {
	    "require_auth": false,
	    "native": {
	      "enabled": true
	    },
	    "file": {
	      "basic": [
	        {
	          "username": "admin",
	          "password": "password",
	          "role": {
	            "type": "super_user",
	            "groups": []
	          }
	        }
	      ],
	      "api-key": [
	        {
	          "api_key": "ABC1234",
	          "role": {
	            "type": "admin",
	            "groups": ["group-uid-1", "group-uid-2"],
	            "apps": ["apps-uid-1", "apps-uid-2"]
	          }
	        }
	      ]
	    }
	  }
	}
	```

-   `strategy`: This specifies retry mechanism for convoy to retry events. Currently supported: `default` (constant time interval) and `exponential-backoff`.

	```json[sample]
	{
	  "strategy": {
	    "type": "default",
	    "default": {
	      "intervalSeconds": 20,
	      "retryLimit": 3
	    }
	  }
	}
	```
-   `signature`: Convoy signs your payload and adds a specific request header specified here. If you omit the header, we default to `X-Convoy-Signature`.

	```json[sample]
	{
	    "signature": {
	        "header": "X-Your-Signature",
	        "hash": "SHA256"
	    }
	}
	```
-   `smtp`: Convoy identifies [dead endpoints](./overview#dead-endpoints) and sends an email to the developers to fix.

	```json[sample]
	{
	    "smtp": {
			"provider": "sendgrid",
			"url": "smtp.sendgrid.net",
			"port": 2525,
			"username": "apikey",
			"password": "api-key-from-sendgrid",
			"from": "support@frain.dev"
		}
	}
	```
-   `tracer` and `new_relic`: Convoy uses [newrelic](https://newrelic.com) for tracing.

	```json[sample]
	{
	  "tracer": {
	    "type": "new_relic"
	  },
	  "new_relic": {
	    "license_key": "012345678909876543210",
	    "app_name": "convoy",
	    "config_enabled": true,
	    "distributed_tracer_enabled": true
	  }
	}
	```
-   `disable_endpoint`: Convoy will disable dead endpoints if this is set to `true`. Defaults to `false`.
-   `sentry`: Convoy uses [sentry](https://sentry.io) for error monitoring.

	```json[sample]
	{
	    "sentry": {
	        "dsn": "sentry-dsn"
	    }
	}
	```
#### Environment Variables

Alternatively, you can configure Convoy using the following environment variables:

- `CONVOY_ENV`
- `SSL`
- `PORT`
- `WORKER_PORT`
- `CONVOY_BASE_URL`
- `CONVOY_DB_TYPE`
- `CONVOY_DB_DSN`
- `CONVOY_SENTRY_DSN`
- `CONVOY_MUTIPLE_TENANTS`
- `CONVOY_LIMITER_PROVIDER`
- `CONVOY_CACHE_PROVIDER`
- `CONVOY_QUEUE_PROVIDER`
- `CONVOY_REDIS_DSN`
- `CONVOY_LOGGER_LEVEL`
- `CONVOY_LOGGER_PROVIDER`
- `CONVOY_SSL_KEY_FILE`
- `CONVOY_SSL_CERT_FILE`
- `CONVOY_STRATEGY_TYPE`
- `CONVOY_SIGNATURE_HASH`
- `CONVOY_DISABLE_ENDPOINT`
- `CONVOY_SIGNATURE_HEADER`
- `CONVOY_INTERVAL_SECONDS`
- `CONVOY_RETRY_LIMIT`
- `CONVOY_SMTP_PROVIDER`
- `CONVOY_SMTP_URL`
- `CONVOY_SMTP_USERNAME`
- `CONVOY_SMTP_PASSWORD`
- `CONVOY_SMTP_FROM`
- `CONVOY_SMTP_PORT`
- `CONVOY_SMTP_REPLY_TO`
- `CONVOY_NEWRELIC_APP_NAME`
- `CONVOY_NEWRELIC_LICENSE_KEY`
- `CONVOY_NEWRELIC_CONFIG_ENABLED`
- `CONVOY_NEWRELIC_DISTRIBUTED_TRACER_ENABLED`
- `CONVOY_REQUIRE_AUTH`
- `CONVOY_BASIC_AUTH_CONFIG`
- `CONVOY_API_KEY_CONFIG`
- `CONVOY_NATIVE_REALM_ENABLED`