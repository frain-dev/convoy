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

	rm -rf .angular/cache dist

	# Run production build
	npm run build

	# Copy build artifacts
	cd ../../../
	mv web/ui/dashboard/dist/* $UIDIR

	echo -n "" > $UIDIR/go_test_stub.txt

	# Build Binary (inject version from VERSION file so UI shows correct version)
	VERSION="$(cat ./VERSION 2>/dev/null | tr -d '\n' | tr -d ' ')"
	LDFLAGS=""
	if [ -n "$VERSION" ]; then
		LDFLAGS="-X github.com/frain-dev/convoy.Version=$VERSION"
	fi
	go build ${LDFLAGS:+-ldflags="$LDFLAGS"} -o convoy ./cmd/*.go
}

buildConvoy
