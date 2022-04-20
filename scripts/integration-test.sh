export TEST_BADGER_DSN=../../db
export TEST_MONGO_DSN=mongodb://localhost:27017/testdb
export TEST_REDIS_DSN=redis://localhost:6379

go test ./... -v --tags integration