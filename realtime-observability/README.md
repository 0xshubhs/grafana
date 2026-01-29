# Real-time Observability Platform

A production-ready, millisecond-level observability system with:

- **gRPC-based telemetry ingestion** (batched, streaming)
- **Lock-free ring buffers** for zero-allocation hot paths
- **WebSocket real-time streaming** at 50-60Hz
- **React live dashboard** with no-reflow charts
- **Prometheus integration** for alerting/trending
- **Grafana dashboards** for historical analysis

## 🏗️ Architecture

```
┌─────────────┐     gRPC Stream     ┌─────────────┐     WebSocket     ┌─────────────┐
│   Agents    │ ──────────────────► │  Aggregator │ ─────────────────► │  Dashboard  │
│  (Go/Rust)  │    20-50ms batch    │   (Go)      │     16ms frames   │  (React)    │
└─────────────┘                     └─────────────┘                    └─────────────┘
                                           │
                                           │ /metrics
                                           ▼
                                    ┌─────────────┐
                                    │ Prometheus  │
                                    │   (1s)      │
                                    └─────────────┘
                                           │
                                           ▼
                                    ┌─────────────┐
                                    │  Grafana    │
                                    │  (Alerts)   │
                                    └─────────────┘
```

## 🚀 Quick Start

### Prerequisites

- Docker & Docker Compose
- Go 1.22+ (for local development)
- Node.js 20+ (for dashboard development)

### Run Everything

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f aggregator
```

### Access Points

| Service    | URL                    | Description              |
| ---------- | ---------------------- | ------------------------ |
| Dashboard  | http://localhost:5173  | Real-time live dashboard |
| Grafana    | http://localhost:3000  | Historical dashboards    |
| Prometheus | http://localhost:9090  | Metrics & queries        |
| WebSocket  | ws://localhost:8080/ws | Direct WS connection     |
| gRPC       | localhost:9000         | Telemetry ingestion      |

## 📦 Components

### Aggregator (Go)

The central hub that:

- Receives gRPC streams from agents
- Maintains lock-free ring buffers per metric
- Broadcasts to WebSocket clients at 60Hz
- Exposes Prometheus metrics

```bash
cd aggregator
go build -o aggregator ./cmd
./aggregator
```

### Agent SDK (Go)

```go
import "github.com/yourorg/agent"

// Create agent
config := agent.DefaultConfig()
config.ServiceName = "my-service"
config.AggregatorAddr = "localhost:9000"

agent, _ := agent.NewAgent(config)
agent.Connect()
agent.Start()

// Track requests
defer agent.TrackRequest()()

// Set gauges
agent.SetGauge("cpu_usage", 45.2)

// Increment counters
agent.IncCounter("requests_total")

// Record histograms
agent.RecordHistogram("latency", 23.5)
```

### Agent SDK (Rust)

```rust
use telemetry_agent::{Agent, Config};

#[tokio::main]
async fn main() {
    let config = Config {
        service_name: "my-service".to_string(),
        aggregator_addr: "http://localhost:9000".to_string(),
        ..Default::default()
    };
    
    let mut agent = Agent::new(config);
    agent.start().await.unwrap();
    
    // Track requests
    let _guard = agent.track_request();
    
    // Set gauges
    agent.set_gauge("cpu_usage", 45.2);
}
```

### Dashboard (React)

WebSocket-first React dashboard with:

- No Redux overhead (Zustand)
- Fixed-size Float32Array buffers
- Recharts for visualization
- Auto-reconnection

```bash
cd dashboard
npm install
npm run dev
```

## 📊 Metrics Schema

### Protobuf Definition

```protobuf
message TelemetryBatch {
  string service = 1;
  string instance = 2;
  repeated Metric metrics = 3;
}

message Metric {
  string name = 1;
  map<string, string> labels = 2;
  repeated MetricSample samples = 3;
}

message MetricSample {
  uint64 timestamp_ns = 1;
  oneof value {
    double gauge = 2;
    uint64 counter = 3;
    Histogram histogram = 4;
  }
}
```

### Standard Metrics

| Metric          | Type      | Description               |
| --------------- | --------- | ------------------------- |
| `latency`       | Histogram | Request latency (ms)      |
| `latency_p50`   | Gauge     | P50 latency               |
| `latency_p95`   | Gauge     | P95 latency               |
| `latency_p99`   | Gauge     | P99 latency               |
| `rps`           | Gauge     | Requests per second       |
| `error_rate`    | Gauge     | Error rate (0-1)          |
| `inflight`      | Gauge     | In-flight requests        |
| `requests_total`| Counter   | Total request count       |
| `errors_total`  | Counter   | Total error count         |

## ⚡ Performance

### Resource Usage (10-12 nodes)

| Component  | RAM       | CPU       |
| ---------- | --------- | --------- |
| Aggregator | ~80-150MB | <0.5 core |
| Agent      | ~5MB      | negligible|
| Dashboard  | browser   | n/a       |
| Prometheus | ~300MB    | bursty    |

### Optimizations

- ✅ Fixed-size ring buffers (no GC pressure)
- ✅ Lock-free writes (single writer per metric)
- ✅ Binary WebSocket frames (JSON fallback)
- ✅ Batched gRPC streams (20-50ms)
- ✅ No allocations in hot paths
- ✅ Float32Array in frontend

## 🔧 Configuration

### Aggregator Environment Variables

```bash
TELEMETRY_API_KEYS=key1,key2,key3  # Comma-separated API keys (empty = disabled)
```

### Agent Configuration

```go
config := agent.Config{
    AggregatorAddr: "localhost:9000",
    ServiceName:    "my-service",
    InstanceID:     "instance-1",
    APIKey:         "secret-key",
    PushInterval:   20 * time.Millisecond,
    BatchSize:      100,
}
```

## 🔒 Security

### Authentication

- API key authentication via gRPC metadata
- Set `TELEMETRY_API_KEYS` environment variable
- Agents include `x-api-key` header

### Future Enhancements

- [ ] mTLS for gRPC
- [ ] JWT authentication for WebSocket
- [ ] Rate limiting
- [ ] Compression (Snappy/Zstd)

## 📈 Scaling

### Horizontal Scaling

```yaml
# docker-compose.scale.yml
services:
  aggregator:
    deploy:
      replicas: 3
```

### Multi-Aggregator Sharding

For 100+ nodes:

1. Deploy multiple aggregators
2. Use consistent hashing by service name
3. Agents connect to assigned aggregator
4. Dashboard connects to all aggregators

## 🧪 Development

### Generate Protobuf

```bash
cd proto
protoc --go_out=../aggregator/internal/ingest \
       --go-grpc_out=../aggregator/internal/ingest \
       telemetry.proto
```

### Run Tests

```bash
# Aggregator
cd aggregator && go test ./...

# Agent
cd agent/go && go test ./...

# Rust agent
cd agent/rust && cargo test
```

### Local Development

```bash
# Terminal 1: Aggregator
cd aggregator && go run ./cmd

# Terminal 2: Dashboard
cd dashboard && npm run dev

# Terminal 3: Example agent
cd agent/go/example && go run main.go
```

## 📝 License

MIT License - See LICENSE file for details.

## 🤝 Contributing

1. Fork the repository
2. Create a feature branch
3. Make your changes
4. Run tests
5. Submit a pull request

---

Built for **millisecond-level observability** with minimal overhead. 🚀
