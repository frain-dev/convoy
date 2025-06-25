#!/bin/bash

# Integration test runner for the loader package
# This script sets up the environment and runs the integration tests

set -e

echo "Setting up environment for loader integration tests..."

# Set default test database environment variables if not already set
export TEST_DB_SCHEME=${TEST_DB_SCHEME:-postgres}
export TEST_DB_OPTIONS=${TEST_DB_OPTIONS:-"sslmode=disable&connect_timeout=30"}
export TEST_DB_HOST=${TEST_DB_HOST:-localhost}
export TEST_DB_USERNAME=${TEST_DB_USERNAME:-subomioluwalana}
export TEST_DB_PASSWORD=${TEST_DB_PASSWORD:-}
export TEST_DB_DATABASE=${TEST_DB_DATABASE:-convoy}
export TEST_DB_PORT=${TEST_DB_PORT:-5432}

export TEST_REDIS_SCHEME=${TEST_REDIS_SCHEME:-redis}
export TEST_REDIS_HOST=${TEST_REDIS_HOST:-localhost}
export TEST_REDIS_PORT=${TEST_REDIS_PORT:-6379}

echo "Database configuration:"
echo "  Host: $TEST_DB_HOST:$TEST_DB_PORT"
echo "  Database: $TEST_DB_DATABASE"
echo "  Username: $TEST_DB_USERNAME"

echo ""
echo "Running loader integration tests..."

# Run the integration tests
go test -tags=integration ./internal/pkg/loader/... -v
#go test -tags=integration ./internal/pkg/loader/... -run ^TestSubscriptionLoaderIntegration/TestIncrementalUpdates/TestUpdateExistingSubscriptions$ -v 

echo ""
echo "Integration tests completed!" 
