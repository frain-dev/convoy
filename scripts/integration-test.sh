#!/bin/bash

export TEST_DB_SCHEME=postgres
export TEST_DB_OPTIONS="sslmode=disable&connect_timeout=30"
export TEST_DB_HOST=localhost
export TEST_DB_USERNAME=postgres
export TEST_DB_PASSWORD=postgres
export TEST_DB_DATABASE=test
export TEST_DB_PORT=5432

export TEST_REDIS_SCHEME=redis
export TEST_REDIS_HOST=localhost
export TEST_REDIS_PORT=6379

# Set CONVOY_ environment variables for the migrate command
export CONVOY_DB_SCHEME=$TEST_DB_SCHEME
export CONVOY_DB_OPTIONS=$TEST_DB_OPTIONS
export CONVOY_DB_HOST=$TEST_DB_HOST
export CONVOY_DB_USERNAME=$TEST_DB_USERNAME
export CONVOY_DB_PASSWORD=$TEST_DB_PASSWORD
export CONVOY_DB_DATABASE=$TEST_DB_DATABASE
export CONVOY_DB_PORT=$TEST_DB_PORT

make integration_tests

make docker_e2e_tests
