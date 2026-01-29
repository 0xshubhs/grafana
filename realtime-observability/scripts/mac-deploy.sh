#!/bin/bash

# =============================================================================
# PURE MAC TELEMETRY DEPLOYMENT (No Docker Required)
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
echo "║      🍎 REAL-TIME OBSERVABILITY - MAC LOCAL DEPLOYMENT 🍎        ║"
echo "╠═══════════════════════════════════════════════════════════════════╣"
echo "║  Aggregator Server  →  gRPC:9000  WS:8080  Metrics:9100          ║"
echo "║  Test Agent 1       →  mac-service-1 (simulated load)            ║"
echo "║  Test Agent 2       →  mac-service-2 (simulated load)            ║"
echo "║  Dashboard          →  http://localhost:5173                     ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Store PIDs for cleanup
PIDS=()

cleanup() {
    echo -e "\n${YELLOW}🧹 Cleaning up all processes...${NC}"
    for pid in "${PIDS[@]}"; do
        kill $pid 2>/dev/null || true
    done
    pkill -f "aggregator/cmd" 2>/dev/null || true
    pkill -f "agent/go/example" 2>/dev/null || true
    echo -e "${GREEN}✅ All processes stopped${NC}"
}

trap cleanup EXIT INT TERM

# =============================================================================
# PHASE 1: Build Aggregator
# =============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}🔨 PHASE 1: Building Aggregator${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"

cd "$PROJECT_DIR/aggregator"
echo -e "${YELLOW}Building aggregator binary...${NC}"
go build -o bin/aggregator ./cmd
echo -e "${GREEN}✅ Aggregator built${NC}"

# =============================================================================
# PHASE 2: Start Aggregator
# =============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}🚀 PHASE 2: Starting Aggregator Server${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"

export NODE_ENV=development
export TELEMETRY_API_KEYS="dev-key-123"
export LOG_LEVEL=debug

./bin/aggregator &
AGGREGATOR_PID=$!
PIDS+=($AGGREGATOR_PID)
echo -e "${GREEN}✅ Aggregator started (PID: $AGGREGATOR_PID)${NC}"

# Wait for aggregator to be ready
echo -e "${YELLOW}Waiting for aggregator...${NC}"
for i in {1..20}; do
    if curl -s http://localhost:8080/health > /dev/null 2>&1; then
        echo -e "${GREEN}✅ Aggregator is healthy!${NC}"
        break
    fi
    if [ $i -eq 20 ]; then
        echo -e "${RED}❌ Aggregator failed to start${NC}"
        exit 1
    fi
    sleep 1
done

cd "$PROJECT_DIR"

# =============================================================================
# PHASE 3: Start Test Agents
# =============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}📡 PHASE 3: Starting Test Agents${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"

cd "$PROJECT_DIR/agent/go"

# Agent 1
export SERVICE_NAME="mac-service-1"
export INSTANCE_ID="node-1"
export AGGREGATOR_ADDR="localhost:9000"
export API_KEY="dev-key-123"
export PUSH_INTERVAL_MS="20"

go run ./example &
AGENT1_PID=$!
PIDS+=($AGENT1_PID)
echo -e "${GREEN}✅ Agent 1 (mac-service-1) started (PID: $AGENT1_PID)${NC}"

sleep 2

# Agent 2
export SERVICE_NAME="mac-service-2"
export INSTANCE_ID="node-2"

go run ./example &
AGENT2_PID=$!
PIDS+=($AGENT2_PID)
echo -e "${GREEN}✅ Agent 2 (mac-service-2) started (PID: $AGENT2_PID)${NC}"

cd "$PROJECT_DIR"

# =============================================================================
# PHASE 4: Start Dashboard
# =============================================================================
echo -e "\n${BLUE}═══════════════════════════════════════════════════════════════════${NC}"
echo -e "${BLUE}🖥️  PHASE 4: Starting Dashboard${NC}"
echo -e "${BLUE}═══════════════════════════════════════════════════════════════════${NC}"

cd "$PROJECT_DIR/dashboard"
export NODE_ENV=development

npm run dev &
DASHBOARD_PID=$!
PIDS+=($DASHBOARD_PID)
echo -e "${GREEN}✅ Dashboard started (PID: $DASHBOARD_PID)${NC}"

sleep 5

cd "$PROJECT_DIR"

# =============================================================================
# STATUS & MONITORING
# =============================================================================
echo -e "\n${CYAN}"
echo "╔═══════════════════════════════════════════════════════════════════╗"
echo "║                    🎉 DEPLOYMENT SUCCESSFUL! 🎉                   ║"
echo "╠═══════════════════════════════════════════════════════════════════╣"
echo "║                                                                   ║"
echo "║  📊 Dashboard:     http://localhost:5173                         ║"
echo "║  🔌 WebSocket:     ws://localhost:8080/ws                        ║"
echo "║  📡 gRPC:          localhost:9000                                ║"
echo "║  📈 Metrics:       http://localhost:9100/metrics                 ║"
echo "║                                                                   ║"
echo "╠═══════════════════════════════════════════════════════════════════╣"
echo "║  ACTIVE SERVICES:                                                 ║"
echo "║    • mac-service-1 (node-1)                                      ║"
echo "║    • mac-service-2 (node-2)                                      ║"
echo "║                                                                   ║"
echo "╠═══════════════════════════════════════════════════════════════════╣"
echo "║  Press Ctrl+C to stop all services                               ║"
echo "╚═══════════════════════════════════════════════════════════════════╝"
echo -e "${NC}"

# Open dashboard in browser
sleep 2
open http://localhost:5173 2>/dev/null || true

# =============================================================================
# CONTINUOUS HEALTH MONITORING
# =============================================================================
echo -e "${YELLOW}Starting continuous health monitoring...${NC}\n"

iteration=0
while true; do
    iteration=$((iteration + 1))
    
    # Health checks
    agg_health=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:8080/health 2>/dev/null || echo "000")
    dash_health=$(curl -s -o /dev/null -w "%{http_code}" http://localhost:5173 2>/dev/null || echo "000")
    
    # Get metrics from aggregator
    metrics=$(curl -s http://localhost:9100/metrics 2>/dev/null)
    service_count=$(echo "$metrics" | grep -c "^service_" 2>/dev/null || echo "0")
    
    timestamp=$(date '+%H:%M:%S')
    
    # Color coded status
    agg_status=$([ "$agg_health" = "200" ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")
    dash_status=$([ "$dash_health" = "200" ] && echo -e "${GREEN}✓${NC}" || echo -e "${RED}✗${NC}")
    
    printf "\r${CYAN}[%s]${NC} #${GREEN}%d${NC} | Aggregator: %s | Dashboard: %s | Metrics: ${GREEN}%s${NC} lines     " \
        "$timestamp" "$iteration" "$agg_status" "$dash_status" "$service_count"
    
    sleep 3
done
