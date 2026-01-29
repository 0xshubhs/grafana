#!/bin/bash

# Build script for the observability platform

set -e

echo "🏗️  Building Real-time Observability Platform..."

# Colors
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
NC='\033[0m' # No Color

# Build aggregator
echo -e "${YELLOW}Building aggregator...${NC}"
cd aggregator
go build -o bin/aggregator ./cmd
echo -e "${GREEN}✓ Aggregator built${NC}"
cd ..

# Build Go agent example
echo -e "${YELLOW}Building Go agent example...${NC}"
cd agent/go/example
go build -o ../../../bin/agent-example .
echo -e "${GREEN}✓ Go agent example built${NC}"
cd ../../..

# Build dashboard
echo -e "${YELLOW}Building dashboard...${NC}"
cd dashboard
npm install
npm run build
echo -e "${GREEN}✓ Dashboard built${NC}"
cd ..

echo -e "${GREEN}🎉 All components built successfully!${NC}"
echo ""
echo "To run locally:"
echo "  1. Start aggregator: ./bin/aggregator"
echo "  2. Start dashboard:  cd dashboard && npm run preview"
echo "  3. Start agent:      ./bin/agent-example"
echo ""
echo "Or use Docker Compose:"
echo "  docker-compose up -d"
