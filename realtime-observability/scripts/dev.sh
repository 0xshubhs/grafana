#!/bin/bash

# Development startup script

set -e

echo "🚀 Starting development environment..."

# Check dependencies
command -v go >/dev/null 2>&1 || { echo "Go is required but not installed."; exit 1; }
command -v node >/dev/null 2>&1 || { echo "Node.js is required but not installed."; exit 1; }
command -v npm >/dev/null 2>&1 || { echo "npm is required but not installed."; exit 1; }

# Start aggregator in background
echo "Starting aggregator..."
cd aggregator
go run ./cmd &
AGGREGATOR_PID=$!
cd ..

# Wait for aggregator to be ready
sleep 2

# Start dashboard in background
echo "Starting dashboard..."
cd dashboard
npm install
npm run dev &
DASHBOARD_PID=$!
cd ..

# Wait for dashboard to be ready
sleep 3

# Start example agent
echo "Starting example agent..."
cd agent/go/example
go run main.go &
AGENT_PID=$!
cd ../../..

echo ""
echo "✅ Development environment started!"
echo ""
echo "Services running:"
echo "  - Aggregator (PID: $AGGREGATOR_PID): gRPC=:9000, WS=:8080, Metrics=:9100"
echo "  - Dashboard  (PID: $DASHBOARD_PID): http://localhost:5173"
echo "  - Agent      (PID: $AGENT_PID): sending telemetry"
echo ""
echo "Press Ctrl+C to stop all services"

# Trap Ctrl+C and kill all processes
trap "kill $AGGREGATOR_PID $DASHBOARD_PID $AGENT_PID 2>/dev/null; exit" SIGINT SIGTERM

# Wait
wait
