# Real-Time Observability Platform - Usage Guide

> Complete reference for every file in this monorepo

---

## 📁 Project Structure Overview

```
realtime-observability/
├── proto/              # Protocol Buffers schema
├── gen/                # Generated Go code from proto
├── aggregator/         # Central telemetry server (Go)
├── agent/              # Telemetry clients (Go + Rust)
├── dashboard/          # React live visualization
├── prometheus/         # Metrics storage config
├── grafana/            # Dashboard provisioning
├── scripts/            # Build & deployment scripts
└── docker-compose*.yml # Container orchestration
```

---

## 🔧 Proto Layer

### `proto/telemetry.proto`
**Purpose**: Defines the gRPC schema for telemetry data

**Key Types**:
```protobuf
MetricSample    # Oneof: gauge, counter, histogram
TelemetryBatch  # Collection of samples with metadata
Ack             # Server acknowledgment with count
```

**Usage**:
```bash
# Regenerate Go code after schema changes
protoc --go_out=./gen --go-grpc_out=./gen proto/telemetry.proto
```

---

### `gen/proto/telemetry.pb.go`
**Purpose**: Auto-generated protobuf message types

**Usage in Go**:
```go
import pb "github.com/yourorg/telemetry/gen/proto"

sample := &pb.MetricSample{
    Name:      "request_latency",
    Timestamp: time.Now().UnixNano(),
    Value:     &pb.MetricSample_Gauge{Gauge: 42.5},
}
```

### `gen/proto/telemetry_grpc.pb.go`
**Purpose**: Auto-generated gRPC client/server stubs

**Usage**:
```go
// Server implements this interface
type TelemetryIngestorServer interface {
    StreamTelemetry(grpc.ClientStreamingServer[TelemetryBatch, Ack]) error
}

// Client uses this
client.StreamTelemetry() // Returns streaming client
```

---

## 🖥️ Aggregator (Server)

### `aggregator/cmd/main.go`
**Purpose**: Application entry point - starts all services

**Environment Variables**:
| Variable | Default | Description |
|----------|---------|-------------|
| `GRPC_PORT` | `9000` | gRPC ingestion port |
| `WS_PORT` | `8080` | WebSocket streaming port |
| `METRICS_PORT` | `9100` | Prometheus metrics port |
| `TELEMETRY_API_KEYS` | - | Comma-separated valid API keys |
| `LOG_LEVEL` | `info` | Logging verbosity |

**Run Locally**:
```bash
cd aggregator
go run ./cmd
```

**Run in Docker**:
```bash
docker-compose -f docker-compose.dev.yml up aggregator
```

---

### `aggregator/internal/ingest/server.go`
**Purpose**: gRPC server handling incoming telemetry streams

**Key Functions**:
```go
NewServer(buffer, hub)           # Create server with dependencies
StreamTelemetry(stream)          # Handle client streaming RPC
```

**How it works**:
1. Receives `TelemetryBatch` from agents via gRPC stream
2. Writes samples to ring buffer (lock-free)
3. Broadcasts to WebSocket hub for live dashboard
4. Returns `Ack` with processed count

---

### `aggregator/internal/buffer/ring.go`
**Purpose**: Lock-free ring buffer for zero-allocation hot path

**Usage**:
```go
ring := buffer.NewRingBuffer(65536) // 64K slots
ring.Write(sample)                   // O(1) write
samples := ring.ReadBatch(1000)      // Bulk read
```

**Performance**: ~2M writes/sec on modern hardware

---

### `aggregator/internal/buffer/registry.go`
**Purpose**: Manages multiple ring buffers per metric type

**Usage**:
```go
registry := buffer.NewRegistry()
registry.GetOrCreate("cpu_usage")  // Auto-creates buffer
registry.GetAll()                  // Iterate all buffers
```

---

### `aggregator/internal/ws/hub.go`
**Purpose**: WebSocket broadcast hub for real-time streaming

**Endpoints**:
| Path | Method | Description |
|------|--------|-------------|
| `/ws` | WS | Live telemetry stream (60Hz) |
| `/health` | GET | Health check endpoint |

**Client Connection**:
```javascript
const ws = new WebSocket('ws://localhost:8080/ws');
ws.onmessage = (e) => {
  const batch = JSON.parse(e.data);
  // batch.samples contains latest metrics
};
```

---

### `aggregator/internal/export/prometheus.go`
**Purpose**: Exposes metrics in Prometheus format

**Endpoint**: `GET /metrics` on port 9100

**Exported Metrics**:
```
telemetry_samples_total{service,instance}     # Counter
telemetry_latency_seconds{service,quantile}   # Summary
telemetry_buffer_size{metric}                 # Gauge
```

---

### `aggregator/internal/auth/auth.go`
**Purpose**: API key validation for gRPC connections

**Usage**:
```go
auth := auth.NewAuth([]string{"key1", "key2"})
if !auth.Validate(apiKey) {
    return status.Error(codes.Unauthenticated, "invalid key")
}
```

---

### `aggregator/Dockerfile`
**Purpose**: Multi-stage build for minimal production image

**Build**:
```bash
# From repo root (context needed for gen/ module)
docker build -f aggregator/Dockerfile -t obs-aggregator .
```

**Exposed Ports**: 9000 (gRPC), 8080 (WS), 9100 (metrics)

---

## 📡 Agent (Go Client)

### `agent/go/agent.go`
**Purpose**: Telemetry collection and streaming client

**Key Types**:
```go
type Config struct {
    ServiceName    string        // Identifies your service
    InstanceID     string        // Unique instance identifier
    AggregatorAddr string        // Server address (host:port)
    APIKey         string        // Authentication key
    PushInterval   time.Duration // Batch send frequency
    BufferSize     int           // Local buffer capacity
}

type Agent struct {
    // Methods:
    Connect() error              // Establish gRPC stream
    Start()                      // Begin background streaming
    Stop()                       // Graceful shutdown
    RecordGauge(name, value, labels)
    RecordCounter(name, delta, labels)
    RecordHistogram(name, value, labels)
}
```

**Usage**:
```go
import agent "github.com/yourorg/agent"

cfg := agent.DefaultConfig()
cfg.ServiceName = "my-api"
cfg.AggregatorAddr = "localhost:9000"

a, _ := agent.NewAgent(cfg)
a.Connect()
a.Start()

// Record metrics
a.RecordGauge("cpu_percent", 45.2, map[string]string{"core": "0"})
a.RecordCounter("requests_total", 1, nil)
a.RecordHistogram("response_time_ms", 23.5, nil)
```

---

### `agent/go/example/main.go`
**Purpose**: Demo agent with simulated workload

**Environment Variables**:
| Variable | Default | Description |
|----------|---------|-------------|
| `SERVICE_NAME` | `mac-local-service` | Service identifier |
| `INSTANCE_ID` | `mac-node-2` | Instance identifier |
| `AGGREGATOR_ADDR` | `localhost:9000` | Server address |
| `API_KEY` | `dev-key-123` | Authentication |
| `PUSH_INTERVAL_MS` | `20` | Push frequency (ms) |

**Run**:
```bash
cd agent/go/example
go run main.go

# Or with custom config
SERVICE_NAME=my-app INSTANCE_ID=prod-1 go run main.go
```

---

### `agent/go/Dockerfile`
**Purpose**: Containerized agent for Docker deployment

**Build & Run**:
```bash
docker build -f agent/go/Dockerfile -t obs-agent .
docker run -e AGGREGATOR_ADDR=host.docker.internal:9000 obs-agent
```

---

### `agent/go/go.mod`
**Purpose**: Go module definition with dependencies

**Key Dependencies**:
- `google.golang.org/grpc v1.64.0` - gRPC client
- `github.com/yourorg/telemetry/gen` - Proto types (local replace)

---

## 🦀 Agent (Rust Client)

### `agent/rust/src/lib.rs`
**Purpose**: Rust telemetry client library

**Usage**:
```rust
use telemetry_agent::{Agent, Config};

let config = Config {
    service_name: "rust-service".into(),
    aggregator_addr: "localhost:9000".into(),
    ..Default::default()
};

let agent = Agent::new(config).await?;
agent.record_gauge("memory_mb", 1024.0, labels!{});
```

### `agent/rust/Cargo.toml`
**Purpose**: Rust package manifest

**Build**:
```bash
cd agent/rust
cargo build --release
```

### `agent/rust/build.rs`
**Purpose**: Compile-time proto generation

Automatically generates Rust types from `proto/telemetry.proto`

---

## 🎨 Dashboard (React)

### `dashboard/src/main.tsx`
**Purpose**: React app entry point

Mounts `<App />` to `#root` with StrictMode

### `dashboard/src/App.tsx`
**Purpose**: Main dashboard layout with charts

**Components Used**:
- `LatencyChart` - P50/P95/P99 latency lines
- `ThroughputChart` - Requests/sec area chart
- `ErrorRateChart` - Error percentage
- `HistogramChart` - Distribution visualization

### `dashboard/src/ws.ts`
**Purpose**: WebSocket client for real-time data

**Usage**:
```typescript
import { wsClient } from './ws';

wsClient.connect('ws://localhost:8080/ws');
wsClient.onMessage((batch) => {
  // Handle incoming telemetry batch
});
```

**Auto-reconnect**: Yes, with exponential backoff

### `dashboard/src/store.ts`
**Purpose**: Zustand state management

**State Shape**:
```typescript
interface TelemetryState {
  samples: MetricSample[];
  services: Map<string, ServiceInfo>;
  latencyP50: number[];
  latencyP95: number[];
  throughput: number[];
  errorRate: number[];
  
  // Actions
  addSamples(batch: TelemetryBatch): void;
  clearOldData(): void;
}
```

**Usage**:
```typescript
import { useTelemetryStore } from './store';

function MyComponent() {
  const latency = useTelemetryStore((s) => s.latencyP50);
  // Renders on latency updates only
}
```

---

### `dashboard/src/charts/*.tsx`
**Purpose**: Recharts-based visualization components

| File | Chart Type | Data Source |
|------|------------|-------------|
| `LatencyChart.tsx` | Multi-line | `latencyP50`, `latencyP95`, `latencyP99` |
| `ThroughputChart.tsx` | Area | `throughput` |
| `ErrorRateChart.tsx` | Line | `errorRate` |
| `HistogramChart.tsx` | Bar | `histogramBuckets` |

**Props**: All charts are self-contained, reading from Zustand store

---

### `dashboard/index.html`
**Purpose**: HTML entry point

Contains `<div id="root">` mount point

### `dashboard/vite.config.ts`
**Purpose**: Vite build configuration

**Dev Server**: Port 5173, HMR enabled

### `dashboard/tailwind.config.js`
**Purpose**: Tailwind CSS configuration

Content paths: `./src/**/*.{ts,tsx}`

### `dashboard/tsconfig.json`
**Purpose**: TypeScript compiler options

Target: ES2020, JSX: react-jsx

### `dashboard/package.json`
**Purpose**: NPM package manifest

**Scripts**:
```bash
npm run dev      # Start dev server (port 5173)
npm run build    # Production build
npm run preview  # Preview production build
npm run lint     # ESLint check
```

---

## 📊 Prometheus

### `prometheus/prometheus.yml`
**Purpose**: Prometheus scrape configuration

**Scrape Targets**:
```yaml
- job_name: 'aggregator'
  static_configs:
    - targets: ['aggregator:9100']
  scrape_interval: 5s
```

**Access**: http://localhost:9090

**Example Queries**:
```promql
# Request rate per service
rate(telemetry_samples_total[1m])

# P99 latency
histogram_quantile(0.99, telemetry_latency_seconds_bucket)

# Buffer utilization
telemetry_buffer_size / telemetry_buffer_capacity
```

---

## 📈 Grafana

### `grafana/provisioning/datasources/datasource.yml`
**Purpose**: Auto-configure Prometheus datasource

Connects to `http://prometheus:9090` on startup

### `grafana/provisioning/dashboards/dashboard.yml`
**Purpose**: Dashboard provisioning config

Loads dashboards from `/etc/grafana/provisioning/dashboards`

### `grafana/dashboards/observability.json`
**Purpose**: Pre-built observability dashboard

**Panels**:
- Latency percentiles (P50/P95/P99)
- Throughput by service
- Error rate trends
- Active connections
- Buffer utilization

**Access**: http://localhost:3001 (admin/admin)

---

## 🐳 Docker Compose

### `docker-compose.yml`
**Purpose**: Production deployment configuration

**Services**: aggregator, prometheus, grafana

**Usage**:
```bash
docker-compose up -d
```

### `docker-compose.dev.yml`
**Purpose**: Development with dual-node setup

**Services**:
| Service | Purpose | Ports |
|---------|---------|-------|
| `aggregator` | Central server | 9000, 8080, 9100 |
| `prometheus` | Metrics storage | 9090 |
| `grafana` | Dashboards | 3001 |
| `test-agent` | Docker telemetry source | - |

**Usage**:
```bash
# Start all services
docker-compose -f docker-compose.dev.yml up -d

# View logs
docker-compose -f docker-compose.dev.yml logs -f aggregator

# Stop all
docker-compose -f docker-compose.dev.yml down
```

---

## 🛠️ Scripts

### `scripts/dev.sh`
**Purpose**: Start full development environment

**Usage**:
```bash
./scripts/dev.sh
```

**What it does**:
1. Builds Docker images
2. Starts aggregator, prometheus, grafana
3. Starts test-agent
4. Launches dashboard dev server

### `scripts/build.sh`
**Purpose**: Build all components

**Usage**:
```bash
./scripts/build.sh [component]

# Examples
./scripts/build.sh           # Build everything
./scripts/build.sh aggregator
./scripts/build.sh agent
./scripts/build.sh dashboard
```

### `scripts/mac-deploy.sh`
**Purpose**: Deploy local Mac agent

**Usage**:
```bash
./scripts/mac-deploy.sh
```

Compiles and runs the Go agent natively on macOS

### `scripts/test-dual-node.sh`
**Purpose**: Verify dual-node telemetry setup

**Usage**:
```bash
./scripts/test-dual-node.sh
```

**Checks**:
- Aggregator health
- Both agents connected
- WebSocket streaming
- Prometheus scraping

---

## 🚀 Quick Start Commands

```bash
# 1. Start infrastructure (Terminal 1)
docker-compose -f docker-compose.dev.yml up -d

# 2. Start dashboard (Terminal 2)
cd dashboard && npm install && npm run dev

# 3. Start Mac agent (Terminal 3)
cd agent/go/example && go run main.go

# 4. Open dashboard
open http://localhost:5173

# 5. Open Grafana
open http://localhost:3001
```

---

## 🔗 Service URLs

| Service | URL | Credentials |
|---------|-----|-------------|
| Dashboard | http://localhost:5173 | - |
| Grafana | http://localhost:3001 | admin/admin |
| Prometheus | http://localhost:9090 | - |
| Aggregator WS | ws://localhost:8080/ws | - |
| Aggregator gRPC | localhost:9000 | API key required |
| Metrics | http://localhost:9100/metrics | - |

---

## 📝 Adding New Metrics

### 1. Define in Proto (if new type needed)
```protobuf
// proto/telemetry.proto
message MyNewMetric {
  string name = 1;
  double value = 2;
}
```

### 2. Regenerate Code
```bash
protoc --go_out=./gen --go-grpc_out=./gen proto/telemetry.proto
```

### 3. Record from Agent
```go
agent.RecordGauge("my_new_metric", 123.45, map[string]string{
    "label1": "value1",
})
```

### 4. Visualize in Dashboard
Add new chart component in `dashboard/src/charts/`

---

## 🐛 Troubleshooting

### Agent can't connect
```bash
# Check aggregator is running
curl http://localhost:8080/health

# Check gRPC port
nc -zv localhost 9000
```

### No data in dashboard
```bash
# Check WebSocket endpoint
wscat -c ws://localhost:8080/ws

# Check agent logs
docker logs obs-test-agent
```

### Prometheus not scraping
```bash
# Check targets
curl http://localhost:9090/api/v1/targets

# Check metrics endpoint
curl http://localhost:9100/metrics
```

---

*Generated: January 2026*
