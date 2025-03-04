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
export TEST_LICENSE_KEY=1D5F41-CD114E-3FBC6B-3843A7-CC0D67-V3

make integration_tests

make docker_e2e_tests
