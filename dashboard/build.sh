#!/usr/bin/env bash
set -euo pipefail

cd "$(dirname "$0")"

echo "Building dashboard..."
npm run build

echo "Copying to Go embed directory..."
rm -rf ../cli/internal/dashboard/static/*
cp -r build/* ../cli/internal/dashboard/static/

echo "✓ Dashboard built and copied"
