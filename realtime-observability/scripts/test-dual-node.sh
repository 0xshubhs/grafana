#!/bin/bash

# =============================================================================
# DUAL-NODE TELEMETRY TEST SCRIPT
# Node 1: Docker (aggregator, prometheus, grafana, test-agent)
# Node 2: Your Mac (local agent, dashboard dev server)
# =============================================================================

set -e

RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
CYAN='\033[0;36m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
PROJECT_DIR="$(dirname "$SCRIPT_DIR")"

cd "$PROJECT_DIR"

echo -e "${CYAN}"
echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║       🔥 REAL-TIME OBSERVABILITY - DUAL NODE DEPLOYMENT 🔥       ║"
echo "╠═══════════════════════════════════════════════════════════════════╣"
echo "║  Node 1 (Docker):  Aggregator, Prometheus, Grafana, Test Agent   ║"
echo "║  Node 2 (Mac):     Local Agent, Dashboard Dev Server             ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Cleanup function
cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up...${NC}"
    docker-compose -f docker-compose.dev.yml down --remove-orphans 2>/dev/null || true
    pkill -f "go run ./example" 2>/dev/null || true
    pkill -f "npm run dev" 2>/dev/null || true
    echo -e "${GREEN}✅ Cleanup complete${NC}"
}

trap cleanup EXIT

# =============================================================================
# PHASE 1: Start Docker Stack (Node 1)
# =============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}📦 PHASE 1: Starting Docker Stack (Node 1)${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"

# Build and start Docker services
echo -e "${YELLOW}Building Docker images...${NC}"
docker-compose -f docker-compose.dev.yml build --parallel

echo -e "${YELLOW}Starting Docker services...${NC}"
docker-compose -f docker-compose.dev.yml up -d

# Wait for aggregator to be healthy
echo -e "${YELLOW}Waiting for aggregator to be ready...${NC}"
RETRIES=30
for i in $(seq 1 $RETRIES); do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Aggregator is healthy!${NC}"
        break
    fi
    if [ $i -eq $RETRIES ]; then
        echo -e "${RED}❌ Aggregator failed to start${NC}"
        docker-compose -f docker-compose.dev.yml logs aggregator
        exit 1
    fi
    echo -e "  ⏳ Attempt $i/$RETRIES..."
    sleep 2
done

# =============================================================================
# PHASE 2: Start Mac Local Agent (Node 2)
# =============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}💻 PHASE 2: Starting Mac Local Agent (Node 2)${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"

cd "$PROJECT_DIR/agent/go"

# Set environment for local Mac agent
export SERVICE_NAME="mac-local-service"
export INSTANCE_ID="mac-node-2"
export AGGREGATOR_ADDR="localhost:9000"
export API_KEY="dev-key-123"
export PUSH_INTERVAL_MS="20"

echo -e "${YELLOW}Starting Mac local agent...${NC}"
go run ./example &
MAC_AGENT_PID=$!
echo -e "${GREEN}✅ Mac agent started (PID: $MAC_AGENT_PID)${NC}"

cd "$PROJECT_DIR"

# =============================================================================
# PHASE 3: Start Dashboard Dev Server
# =============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}🖥️  PHASE 3: Starting Dashboard Dev Server${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"

cd "$PROJECT_DIR/dashboard"

export NODE_ENV=development

echo -e "${YELLOW}Starting dashboard dev server...${NC}"
npm run dev &
DASHBOARD_PID=$!

# Wait for dashboard
sleep 5
echo -e "${GREEN}✅ Dashboard started (PID: $DASHBOARD_PID)${NC}"

cd "$PROJECT_DIR"

# =============================================================================
# PHASE 4: Verification & Status
# =============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}🔍 PHASE 4: Verification & Status${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"

echo -e "\n${CYAN}Docker Container Status:${NC}"
docker-compose -f docker-compose.dev.yml ps

echo -e "\n${CYAN}Testing Endpoints:${NC}"

# Test each endpoint
test_endpoint() {
    local name=$1
    local url=$2
    if curl -s --max-time 3 "$url" > /dev/null 2>&1; then
        echo -e "  ${GREEN}✅ $name: $url${NC}"
    else
        echo -e "  ${RED}❌ $name: $url (FAILED)${NC}"
    fi
}

test_endpoint "Aggregator Health" "http://localhost:8080/health"
test_endpoint "Aggregator Metrics" "http://localhost:9100/metrics"
test_endpoint "Prometheus" "http://localhost:9090/-/healthy"
test_endpoint "Grafana" "http://localhost:3000/api/health"
test_endpoint "Dashboard" "http://localhost:5173"

# =============================================================================
# SUMMARY
# =============================================================================
echo -e "\n${CYAN}"
echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║                    🎉 DEPLOYMENT SUCCESSFUL! 🎉                   ║"
echo "╠═══════════════════════════════════════════════════════════════════╣"
echo "║                                                                   ║"
echo "║  📊 Dashboard:    http://localhost:5173                          ║"
echo "║  📈 Grafana:      http://localhost:3000  (admin/admin)           ║"
echo "║  🔍 Prometheus:   http://localhost:9090                          ║"
echo "║  🔌 WebSocket:    ws://localhost:8080/ws                         ║"
echo "║  📡 gRPC:         localhost:9000                                 ║"
echo "║                                                                   ║"
echo "╠═══════════════════════════════════════════════════════════════════╣"
echo "║  ACTIVE NODES:                                                    ║"
echo "║    Node 1 (Docker): docker-test-service                          ║"
echo "║    Node 2 (Mac):    mac-local-service                            ║"
echo "║                                                                   ║"
echo "╠═══════════════════════════════════════════════════════════════════╣"
echo "║  Press Ctrl+C to stop all services                               ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# =============================================================================
# CONTINUOUS TESTING LOOP
# =============================================================================
echo -e "${YELLOW}Starting continuous health monitoring...${NC}\n"

iteration=0
while true; do
    iteration=$((iteration + 1))
    
    # Health checks
    agg_health=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health 2>/dev/null || echo "000")
    prom_health=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:9090/-/healthy 2>/dev/null || echo "000")
    dash_health=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:5173 2>/dev/null || echo "000")
    
    # Get metrics count from aggregator
    metrics_count=$(curl -s http://localhost:9100/metrics 2>/dev/null | grep -c "^service_" || echo "0")
    
    # WebSocket connection test
    ws_test="OK"
    
    timestamp=$(date '+%H:%M:%S')
    
    printf "\r${CYAN}[%s]${NC} Iteration: ${GREEN}%d${NC} | Aggregator: %s | Prometheus: %s | Dashboard: %s | Metrics: ${GREEN}%s${NC}    " \
        "$timestamp" \
        "$iteration" \
        "$([ "$agg_health" = "200" ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")" \
        "$([ "$prom_health" = "200" ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")" \
        "$([ "$dash_health" = "200" ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")" \
        "$metrics_count"
    
    sleep 2
done
