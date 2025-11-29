#!/usr/bin/env bash

#MISE description="Install dashboard dependencies"
#MISE dir="{{ config_root }}/web/ui/dashboard"
#MISE sources=["package.json", "package-lock.json"]
#MISE outputs=["node_modules/**/*"]

set -e

echo "📦 Installing dashboard dependencies..."
npm install
