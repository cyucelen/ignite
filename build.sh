#!/bin/bash
set -e

echo "🔨 Building Ignite Container API with Web UI"
echo "=============================================="

# Step 1: Build React frontend
echo ""
echo "📦 Step 1/3: Building React frontend..."
cd ui
npm run build
cd ..
echo "✅ Frontend build complete: ui/build/"

# Step 2: Ensure embed directory exists
echo ""
echo "📁 Step 2/3: Preparing embed directory..."
mkdir -p internal/ui/build
rm -rf internal/ui/build/*
cp -r ui/build/* internal/ui/build/
echo "✅ Frontend artifacts copied to internal/ui/build/"

# Step 3: Build Go binary with embedded frontend
echo ""
echo "🔧 Step 3/3: Building Go binary with embedded frontend..."
go build -o bin/ignite-api cmd/api/main.go
echo "✅ Go binary built: bin/ignite-api"

echo ""
echo "=============================================="
echo "✅ Build complete!"
echo ""
echo "Run the application:"
echo "  ./bin/ignite-api"
echo ""
echo "Then visit:"
echo "  http://localhost:3000 (Web UI)"
echo "  http://localhost:3000/api/health (API)"
