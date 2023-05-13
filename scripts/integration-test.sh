export CONVOY_DB_SCHEME=postgres
export CONVOY_DB_HOST=localhost
export CONVOY_DB_USERNAME=postgres
export CONVOY_DB_PASSWORD=postgres
export CONVOY_DB_DATABASE=test
export CONVOY_DB_PORT=5432

export CONVOY_REDIS_SCHEME=redis
export CONVOY_REDIS_HOST=localhost
export CONVOY_REDIS_PORT=6379

export CONVOY_TYPESENSE_HOST=http://localhost:8108
export CONVOY_TYPESENSE_API_KEY=some-api-key
export CONVOY_SEARCH_TYPE=typesense

make integration_tests
