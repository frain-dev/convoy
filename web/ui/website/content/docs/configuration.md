---
title: Configuration
description: 'Convoy Configuration'
id: configuration
order: 3
---

# Configuration

There are two ways to configure Convoy - `convoy.json` or `environment variables`. An example configuration is shown below:

```json[Sample Config]
{
  "database": {
    "dsn": "mongodb://root:rootpassword@localhost:27037"
  },
  "queue": {
    "type": "redis",
    "redis": {
      "dsn": "redis://localhost:8379"
    }
  },
  "server": {
    "http": {
      "port": 5005
    }
  },
  "auth": {
    "type": "none"
  },
  "strategy": {
    "type": "default",
    "default": {
      "intervalSeconds": 125,
      "retryLimit": 15
    }
  },
  "ui": {
      "type": "none"
  }
}
```

## Parameters

-   `environment`: Configure which environment configure is running on. Defaults `development`.
-   `database`: Configures the database DSN Convoy needs to persistent events. Currently supported databases: `mongodb`, planned: `disk`, `postgres`, `dynamodb`.
-   `queue`: Essentially, Convoy is a dedicated task queue for webhooks. This configures a queueing backend to use. Currently supported queueing backends: `redis`, planned: `in-memory`, `sqs`, `rabbitmq`.
-   `port`: Specifies which port Convoy should run on.
-   `auth`: This specifies authentication mechanism used to authenticate against Convoy's public API.
    -   `type`: Convoy supports two authentication mechanisms - `none`: free access, and `basic`: `username` & `password`.
    ```json[sample]
    {
        "auth": {
            "type": "basic",
            "basic": {
                "username": "<username>",
                "password": "<password>"
            }
        }
    }
    ```
-   `strategy`: This specifies retry mechanism for convoy to retry events. Currently supported: `constant-time-interval`, default: `constant-time-interval`, planned: `exponential-backoff`.

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

-   `ui`: Convoy ships with a UI. This blocks configures authentication for the UI.
    -   `type`: Convoy supports two authentication mechanisms - `none`: free access, and `basic`: `username` & `password`.
    ```json[sample]
    {
        "ui": {
            "type": "basic",
            "basic": [
                {
                    "username": "user1",
                    "password": "password1"
                }
            ],
            "jwtKey": "<insert-secret-key>",
            "jwtTokenExpirySeconds": 3600
        }
    }
    ```
-   `signature`: Convoy signs your payload and adds a specific request header specified here. If you omit the header, we default to `X-Convoy-Signature`.

```json[sample]
{
    "signature": {
        "header": "X-Company-Name-Signature",
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
		"password": "<api-key-from-sendgrid>",
		"from": "support@frain.dev"
	}
}
```

-   `disable_endpoint`: Configure Convoy to disable dead endpoints or not. Defaults to `false`.
-   `sentry`: Convoy uses [sentry](https://sentry.io) for error monitoring.

```json[sample]
{
    "sentry": {
        "dsn": "<insert-sentry-dsn>"
    }
}
```

## Environment Variables

Alternatively, you can configure Convoy using the following environment variables:

- `CONVOY_ENV`
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
- `SSL`
- `PORT`
- `WORKER_PORT`
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
