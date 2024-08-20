#!/bin/bash

helpFunc() {
	echo ""
	echo "Usage: $0 -b"
	echo -e "\t-b Specify the build flag (either ce or ee)"
	exit 1
}

buildUi() {
    # Enter UI directory
    cd ./web/ui/dashboard || exit 1

    # Install dependencies
    npm i

    # Run production build
    if [[ "$build" == "ce" ]]; then
    		npm run build
    	elif [[ "$build" == "ee" ]]; then
    		npm run build:ee
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
			buildUi
			;;
		:) helpFunc ;;
	esac
done

if [ -z "$build" ]; then
	echo "Missing required argument: -b build">&2
	exit 1
fi
