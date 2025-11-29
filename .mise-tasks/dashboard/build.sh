#!/usr/bin/env bash

#MISE description="Build the Angular dashboard application"
#MISE dir="{{ config_root }}"
#MISE sources=["/web/ui/dashboard/**/*"]

set -e


echo -e "Building Convoy ..."

# Remove build folder
rm -rf ./api/ui/build

mkdir -p ./api/ui/build

cd ./web/ui/dashboard

# Install dependencies
npm i

# Run production build
npm run build

# Go back to root
cd ../../../

# Copy build artifacts
mv web/ui/dashboard/dist/* ./api/ui/build