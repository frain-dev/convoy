module github.com/frain-dev/convoy

go 1.21.4

toolchain go1.21.6

require (
	cloud.google.com/go/pubsub v1.33.0
	github.com/Subomi/go-authz v0.2.0
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d
	github.com/aws/aws-sdk-go v1.44.327
	github.com/danvixent/asynqmon v0.7.3
	github.com/dchest/uniuri v0.0.0-20200228104902-7aecb25e1fe5
	github.com/dop251/goja v0.0.0-20230828202809-3dbe69dd2b8e
	github.com/dop251/goja_nodejs v0.0.0-20230821135201-94e508132562
	github.com/fatih/structs v1.1.0
	github.com/felixge/httpsnoop v1.0.3
	github.com/getkin/kin-openapi v0.80.0
	github.com/getsentry/sentry-go v0.25.0
	github.com/getsentry/sentry-go/otel v0.25.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi/v5 v5.0.10
	github.com/go-chi/render v1.0.1
	github.com/go-redis/cache/v9 v9.0.0
	github.com/go-redis/redis_rate/v10 v10.0.1
	github.com/go-redsync/redsync/v4 v4.8.1
	github.com/golang-jwt/jwt v3.2.2+incompatible
	github.com/golang/mock v1.6.0
	github.com/gorilla/websocket v1.5.0
	github.com/hibiken/asynq v0.24.1
	github.com/hibiken/asynq/x v0.0.0-20221219051101-0b8cfad70341
	github.com/jackc/pgx v3.6.2+incompatible
	github.com/jarcoal/httpmock v1.3.1
	github.com/jaswdr/faker v1.10.2
	github.com/jmoiron/sqlx v1.3.5
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/lib/pq v1.10.7
	github.com/mattn/go-sqlite3 v1.14.16
	github.com/mixpanel/mixpanel-go v1.2.1
	github.com/newrelic/go-agent/v3 v3.20.4
	github.com/newrelic/go-agent/v3/integrations/nrpq v1.1.1
	github.com/oklog/ulid/v2 v2.1.0
	github.com/pkg/errors v0.9.1
	github.com/posthog/posthog-go v0.0.0-20240115103626-fbd687c18571
	github.com/prometheus/client_golang v1.16.0
	github.com/r3labs/diff/v3 v3.0.1
	github.com/rabbitmq/amqp091-go v1.9.0
	github.com/redis/go-redis/extra/redisotel/v9 v9.0.5
	github.com/redis/go-redis/v9 v9.1.0
	github.com/riandyrn/otelchi v0.5.1
	github.com/rubenv/sql-migrate v1.3.0
	github.com/sebdah/goldie/v2 v2.5.3
	github.com/segmentio/kafka-go v0.4.42
	github.com/sirupsen/logrus v1.9.3
	github.com/slack-go/slack v0.10.2
	github.com/spf13/cobra v1.2.1
	github.com/spf13/pflag v1.0.5
	github.com/stretchr/testify v1.8.4
	github.com/subomi/requestmigrations v0.4.0
	github.com/swaggo/swag v1.16.3
	github.com/tidwall/gjson v1.16.0
	github.com/uptrace/opentelemetry-go-extra/otelsql v0.2.3
	github.com/uptrace/opentelemetry-go-extra/otelsqlx v0.2.3
	github.com/xdg-go/pbkdf2 v1.0.0
	go.opentelemetry.io/otel v1.21.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace v1.21.0
	go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc v1.21.0
	go.opentelemetry.io/otel/sdk v1.21.0
	go.opentelemetry.io/otel/trace v1.21.0
	golang.org/x/crypto v0.18.0
	google.golang.org/api v0.128.0
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df

)

require (
	github.com/Masterminds/semver/v3 v3.2.1 // indirect
	github.com/cenkalti/backoff/v4 v4.2.1 // indirect
	github.com/cockroachdb/apd v1.1.0 // indirect
	github.com/dlclark/regexp2 v1.7.0 // indirect
	github.com/go-logr/logr v1.4.1 // indirect
	github.com/go-logr/stdr v1.2.2 // indirect
	github.com/go-redis/redis/v7 v7.4.1 // indirect
	github.com/go-redis/redis/v8 v8.11.5 // indirect
	github.com/go-sourcemap/sourcemap v2.1.3+incompatible // indirect
	github.com/gofrs/uuid v4.4.0+incompatible // indirect
	github.com/gomodule/redigo v2.0.0+incompatible // indirect
	github.com/google/pprof v0.0.0-20230817174616-7a8ec2ada47b // indirect
	github.com/google/s2a-go v0.1.5 // indirect
	github.com/grpc-ecosystem/grpc-gateway/v2 v2.16.0 // indirect
	github.com/guregu/null/v5 v5.0.0 // indirect
	github.com/hashicorp/errwrap v1.1.0 // indirect
	github.com/hashicorp/go-multierror v1.1.1 // indirect
	github.com/jackc/fake v0.0.0-20150926172116-812a484cc733 // indirect
	github.com/pierrec/lz4/v4 v4.1.18 // indirect
	github.com/redis/go-redis/extra/rediscmd/v9 v9.0.5 // indirect
	github.com/tidwall/match v1.1.1 // indirect
	github.com/tidwall/pretty v1.2.1 // indirect
	github.com/xdg-go/scram v1.1.2 // indirect
	github.com/xdg-go/stringprep v1.0.4 // indirect
	go.opentelemetry.io/contrib v1.0.0 // indirect
	go.opentelemetry.io/otel/metric v1.21.0 // indirect
	go.opentelemetry.io/proto/otlp v1.0.0 // indirect
	google.golang.org/genproto/googleapis/api v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/genproto/googleapis/rpc v0.0.0-20230822172742-b8732ec3820d // indirect
)

require (
	cloud.google.com/go v0.110.7 // indirect
	cloud.google.com/go/compute v1.23.0 // indirect
	cloud.google.com/go/compute/metadata v0.2.3 // indirect
	cloud.google.com/go/iam v1.1.2 // indirect
	github.com/KyleBanks/depth v1.2.1 // indirect
	github.com/beorn7/perks v1.0.1 // indirect
	github.com/cespare/xxhash/v2 v2.2.0 // indirect
	github.com/davecgh/go-spew v1.1.2-0.20180830191138-d8f796af33cc // indirect
	github.com/dgryski/go-rendezvous v0.0.0-20200823014737-9f7001d12a5f // indirect
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/go-gorp/gorp/v3 v3.0.2 // indirect
	github.com/go-openapi/jsonpointer v0.20.2 // indirect
	github.com/go-openapi/jsonreference v0.20.4 // indirect
	github.com/go-openapi/spec v0.20.14 // indirect
	github.com/go-openapi/swag v0.22.9 // indirect
	github.com/golang/groupcache v0.0.0-20210331224755-41bb18bfe9da // indirect
	github.com/golang/protobuf v1.5.3 // indirect
	github.com/google/uuid v1.3.1
	github.com/googleapis/enterprise-certificate-proxy v0.2.5 // indirect
	github.com/googleapis/gax-go/v2 v2.11.0 // indirect
	github.com/gorilla/mux v1.8.0 // indirect
	github.com/inconshreveable/mousetrap v1.0.0 // indirect
	github.com/jmespath/go-jmespath v0.4.0 // indirect
	github.com/josharian/intern v1.0.0 // indirect
	github.com/klauspost/compress v1.17.1 // indirect
	github.com/mailru/easyjson v0.7.7 // indirect
	github.com/matttproud/golang_protobuf_extensions v1.0.4 // indirect
	github.com/nsf/jsondiff v0.0.0-20210926074059-1e845ec5d249
	github.com/pmezard/go-difflib v1.0.1-0.20181226105442-5d4384ee4fb2 // indirect
	github.com/prometheus/client_model v0.3.0 // indirect
	github.com/prometheus/common v0.42.0 // indirect
	github.com/prometheus/procfs v0.10.1 // indirect
	github.com/robfig/cron/v3 v3.0.1 // indirect
	github.com/sergi/go-diff v1.0.0 // indirect
	github.com/spf13/cast v1.5.1 // indirect
	github.com/vmihailenco/go-tinylfu v0.2.2 // indirect
	github.com/vmihailenco/msgpack/v5 v5.3.5
	github.com/vmihailenco/tagparser/v2 v2.0.0 // indirect
	go.opencensus.io v0.24.0 // indirect
	golang.org/x/net v0.20.0 // indirect
	golang.org/x/oauth2 v0.11.0 // indirect
	golang.org/x/sync v0.6.0 // indirect
	golang.org/x/sys v0.17.0 // indirect
	golang.org/x/text v0.14.0 // indirect
	golang.org/x/time v0.3.0 // indirect
	golang.org/x/tools v0.17.0 // indirect
	google.golang.org/appengine v1.6.7 // indirect
	google.golang.org/genproto v0.0.0-20230822172742-b8732ec3820d // indirect
	google.golang.org/grpc v1.59.0 // indirect
	google.golang.org/protobuf v1.31.0 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/yaml.v2 v2.4.0 // indirect
	gopkg.in/yaml.v3 v3.0.1 // indirect
)
