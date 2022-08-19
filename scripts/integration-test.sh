export TEST_MONGO_DSN=mongodb://localhost:27017/testdb
export TEST_REDIS_DSN=redis://localhost:6379
export TEST_CONVOY_TYPESENSE_HOST=http://localhost:8108
export TEST_CONVOY_TYPESENSE_API_KEY=some-api-key
export TEST_CONVOY_SEARCH_TYPE=typesense

make integration_tests