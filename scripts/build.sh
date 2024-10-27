#!/bin/bash

# export CGO_ENABLED=0
# export GOOS=linux
# export GOARCH=arm64

buildConvoy() {
    echo "Building Convoy ..."

	# Build UI.
	UIDIR="api/ui/build"

	# Remove build folder
	rm -rf $UIDIR

	# Recreate build folder
	mkdir $UIDIR

	# Enter UI directory
	cd ./web/ui/dashboard

	# Install dependencies
	npm i

	# Run production build
	npm run build

	# Copy build artifacts
	cd ../../../
	mv web/ui/dashboard/dist/* $UIDIR

	# Build Binary
	go build -o convoy ./cmd/*.go
}

buildConvoy
