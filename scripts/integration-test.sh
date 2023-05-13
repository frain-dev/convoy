export TEST_DB_SCHEME=postgres
export TEST_DB_HOST=localhost
export TEST_DB_USERNAME=postgres
export TEST_DB_PASSWORD=postgres
export TEST_DB_DATABASE=test
export TEST_DB_PORT=5432

export TEST_REDIS_SCHEME=redis
export TEST_REDIS_HOST=localhost
export TEST_REDIS_PORT=6379

export TEST_TYPESENSE_HOST=http://localhost:8108
export TEST_TYPESENSE_API_KEY=some-api-key
export TEST_SEARCH_TYPE=typesense

make integration_tests
