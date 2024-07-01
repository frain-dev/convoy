#!/bin/bash

helpFunc() {
	echo ""
	echo "Usage: $0 -b"
	echo -e "\t-b Specify the build flag (either ce or ee)"
	exit 1
}


buildConvoy() {
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
	if [[ "$build" == "ce" ]]; then
		npm run build
	elif [[ "$build" == "ee" ]]; then
		npm run build:ee
	fi

	# Copy build artifacts
	cd ../../../
	mv web/ui/dashboard/dist/* $UIDIR

    export CGO_ENABLED=0
    export GOOS=linux
    export GOARCH=arm64

	# Build Binary
	if [[ "$build" == "ce" ]]; then
		go build -o convoy ./cmd/*.go
	elif [[ "$build" == "ee" ]]; then
		go build -o convoy-ee ./ee/cmd/*.go
	fi
}

while getopts ":b:" opt; do
	case "$opt" in
		b)
			build="$OPTARG"

			if [[ "$build" == "ce" ]]; then
				echo "Building Convoy Community Edition ..."
			elif [[ "$build" == "ee" ]]; then
				echo "Building Convoy Enterprise Edition ..."
			else
				helpFunc
			fi

			# Build Convoy
			echo ""
			buildConvoy
			;;
		?) helpFunc ;;
		:) helpFunc ;;
	esac
done

if [ -z "$build" ]; then
	echo "Missing required argument: -b build">&2
	exit 1
fi
