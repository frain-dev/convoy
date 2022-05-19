module github.com/frain-dev/convoy

go 1.16

replace github.com/dgraph-io/ristretto v0.1.0 => github.com/frain-dev/ristretto v0.1.1

require (
	github.com/asaskevich/govalidator v0.0.0-20210307081110-f21760c49a8d
	github.com/dchest/uniuri v0.0.0-20200228104902-7aecb25e1fe5
	github.com/dgraph-io/badger/v3 v3.2103.1
	github.com/dgryski/go-farm v0.0.0-20200201041132-a6ae2369ad13 // indirect
	github.com/felixge/httpsnoop v1.0.2
	github.com/frain-dev/disq v0.1.7
	github.com/fsnotify/fsnotify v1.5.1 // indirect
	github.com/getkin/kin-openapi v0.80.0
	github.com/getsentry/sentry-go v0.11.0
	github.com/ghodss/yaml v1.0.0
	github.com/go-chi/chi/v5 v5.0.3
	github.com/go-chi/httprate v0.5.2
	github.com/go-chi/render v1.0.1
	github.com/go-redis/cache/v8 v8.4.3
	github.com/go-redis/redis/v8 v8.11.4
	github.com/go-redis/redis_rate/v9 v9.1.2
	github.com/gobeam/mongo-go-pagination v0.0.7
	github.com/golang/mock v1.6.0
	github.com/google/go-cmp v0.5.7 // indirect
	github.com/google/uuid v1.3.0
	github.com/jarcoal/httpmock v1.0.8
	github.com/jaswdr/faker v1.10.2
	github.com/jeremywohl/flatten v1.0.1
	github.com/kelseyhightower/envconfig v1.4.0
	github.com/klauspost/compress v1.15.4 // indirect
	github.com/mattn/go-colorable v0.1.11 // indirect
	github.com/mgutz/ansi v0.0.0-20200706080929-d51e80ef957d // indirect
	github.com/newrelic/go-agent/v3 v3.15.2
	github.com/newrelic/go-agent/v3/integrations/nrlogrus v1.0.1
	github.com/newrelic/go-agent/v3/integrations/nrmongo v1.0.2
	github.com/olekukonko/tablewriter v0.0.5
	github.com/onsi/gomega v1.19.0 // indirect
	github.com/pkg/errors v0.9.1
	github.com/prometheus/client_golang v1.7.1
	github.com/sebdah/goldie/v2 v2.5.3
	github.com/sirupsen/logrus v1.8.1
	github.com/slack-go/slack v0.10.2
	github.com/spf13/cobra v1.2.1
	github.com/stretchr/testify v1.7.1
	github.com/swaggo/swag v1.7.3
	github.com/timshannon/badgerhold/v4 v4.0.2
	github.com/typesense/typesense-go v0.4.0
	github.com/x-cray/logrus-prefixed-formatter v0.5.2
	github.com/xdg-go/pbkdf2 v1.0.0
	github.com/youmark/pkcs8 v0.0.0-20201027041543-1326539a0a0a // indirect
	go.mongodb.org/mongo-driver v1.8.4
	golang.org/x/crypto v0.0.0-20211215153901-e495a2d5b3d3
	golang.org/x/tools v0.1.7 // indirect
	gopkg.in/alexcesaro/quotedprintable.v3 v3.0.0-20150716171945-2caba252f4dc // indirect
	gopkg.in/gomail.v2 v2.0.0-20160411212932-81ebce5c23df
)
