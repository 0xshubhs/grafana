package agent

import (
	"context"
	"log"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	pb "github.com/yourorg/telemetry/gen/proto"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

// Config holds agent configuration
type Config struct {
	AggregatorAddr string
	ServiceName    string
	InstanceID     string
	APIKey         string
	PushInterval   time.Duration
	BatchSize      int
}

// DefaultConfig returns default agent configuration
func DefaultConfig() Config {
	return Config{
		AggregatorAddr: "localhost:9000",
		ServiceName:    "default",
		InstanceID:     generateInstanceID(),
		PushInterval:   20 * time.Millisecond,
		BatchSize:      100,
	}
}

// Agent collects and pushes telemetry to the aggregator
type Agent struct {
	config Config
	conn   *grpc.ClientConn
	stream grpc.ClientStreamingClient[pb.TelemetryBatch, pb.Ack]

	// Metric collectors
	gauges     map[string]*float64
	counters   map[string]*uint64
	histograms map[string]*Histogram
	mu         sync.RWMutex

	// Inflight tracking
	inflight atomic.Int64

	// Control
	ctx    context.Context
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// Histogram tracks latency distribution
type Histogram struct {
	bounds []float64
	counts []uint64
	mu     sync.Mutex
}

// NewHistogram creates a new histogram with default latency bounds
func NewHistogram() *Histogram {
	return &Histogram{
		bounds: []float64{1, 5, 10, 25, 50, 100, 250, 500, 1000, 2500, 5000, 10000},
		counts: make([]uint64, 13), // +1 for overflow bucket
	}
}

// Record records a value in the histogram
func (h *Histogram) Record(value float64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	for i, bound := range h.bounds {
		if value <= bound {
			h.counts[i]++
			return
		}
	}
	h.counts[len(h.counts)-1]++ // Overflow bucket
}

// Snapshot returns current histogram state and resets
func (h *Histogram) Snapshot() ([]float64, []uint64) {
	h.mu.Lock()
	defer h.mu.Unlock()

	bounds := make([]float64, len(h.bounds))
	counts := make([]uint64, len(h.counts))
	copy(bounds, h.bounds)
	copy(counts, h.counts)

	// Reset counts
	for i := range h.counts {
		h.counts[i] = 0
	}

	return bounds, counts
}

// NewAgent creates a new telemetry agent
func NewAgent(config Config) (*Agent, error) {
	ctx, cancel := context.WithCancel(context.Background())

	agent := &Agent{
		config:     config,
		gauges:     make(map[string]*float64),
		counters:   make(map[string]*uint64),
		histograms: make(map[string]*Histogram),
		ctx:        ctx,
		cancel:     cancel,
	}

	return agent, nil
}

// Connect establishes connection to the aggregator
func (a *Agent) Connect() error {
	var err error
	a.conn, err = grpc.NewClient(
		a.config.AggregatorAddr,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return err
	}

	client := pb.NewTelemetryIngestorClient(a.conn)

	// Add API key to context if configured
	ctx := a.ctx
	if a.config.APIKey != "" {
		ctx = metadata.AppendToOutgoingContext(ctx, "x-api-key", a.config.APIKey)
	}

	a.stream, err = client.StreamTelemetry(ctx)
	if err != nil {
		return err
	}

	log.Printf("Connected to aggregator at %s", a.config.AggregatorAddr)
	return nil
}

// Start begins the metric collection and push loop
func (a *Agent) Start() {
	a.wg.Add(1)
	go a.pushLoop()
}

// Stop gracefully stops the agent
func (a *Agent) Stop() {
	a.cancel()
	a.wg.Wait()

	if a.stream != nil {
		a.stream.CloseAndRecv()
	}
	if a.conn != nil {
		a.conn.Close()
	}
}

// pushLoop periodically sends metrics to the aggregator
func (a *Agent) pushLoop() {
	defer a.wg.Done()

	ticker := time.NewTicker(a.config.PushInterval)
	defer ticker.Stop()

	for {
		select {
		case <-a.ctx.Done():
			return
		case <-ticker.C:
			batch := a.collectMetrics()
			if len(batch.Metrics) > 0 {
				if err := a.stream.Send(batch); err != nil {
					log.Printf("Failed to send batch: %v", err)
				}
			}
		}
	}
}

// collectMetrics gathers all current metrics into a batch
func (a *Agent) collectMetrics() *pb.TelemetryBatch {
	a.mu.RLock()
	defer a.mu.RUnlock()

	now := uint64(time.Now().UnixNano())
	metrics := make([]*pb.Metric, 0)

	// Collect gauges
	for name, val := range a.gauges {
		metrics = append(metrics, &pb.Metric{
			Name: name,
			Samples: []*pb.MetricSample{
				{
					TimestampNs: now,
					Value:       &pb.MetricSample_Gauge{Gauge: *val},
				},
			},
		})
	}

	// Collect counters
	for name, val := range a.counters {
		metrics = append(metrics, &pb.Metric{
			Name: name,
			Samples: []*pb.MetricSample{
				{
					TimestampNs: now,
					Value:       &pb.MetricSample_Counter{Counter: *val},
				},
			},
		})
	}

	// Collect histograms
	for name, hist := range a.histograms {
		bounds, counts := hist.Snapshot()
		metrics = append(metrics, &pb.Metric{
			Name: name,
			Samples: []*pb.MetricSample{
				{
					TimestampNs: now,
					Value: &pb.MetricSample_Histogram{
						Histogram: &pb.Histogram{
							Bounds: bounds,
							Counts: counts,
						},
					},
				},
			},
		})
	}

	// Add inflight metric
	metrics = append(metrics, &pb.Metric{
		Name: "inflight",
		Samples: []*pb.MetricSample{
			{
				TimestampNs: now,
				Value:       &pb.MetricSample_Gauge{Gauge: float64(a.inflight.Load())},
			},
		},
	})

	return &pb.TelemetryBatch{
		Service:  a.config.ServiceName,
		Instance: a.config.InstanceID,
		Metrics:  metrics,
	}
}

// --- Metric Recording Methods ---

// SetGauge sets a gauge metric value
func (a *Agent) SetGauge(name string, value float64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.gauges[name] == nil {
		v := value
		a.gauges[name] = &v
	} else {
		*a.gauges[name] = value
	}
}

// IncCounter increments a counter metric
func (a *Agent) IncCounter(name string) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.counters[name] == nil {
		v := uint64(1)
		a.counters[name] = &v
	} else {
		*a.counters[name]++
	}
}

// AddCounter adds to a counter metric
func (a *Agent) AddCounter(name string, delta uint64) {
	a.mu.Lock()
	defer a.mu.Unlock()

	if a.counters[name] == nil {
		a.counters[name] = &delta
	} else {
		*a.counters[name] += delta
	}
}

// RecordHistogram records a value in a histogram
func (a *Agent) RecordHistogram(name string, value float64) {
	a.mu.Lock()
	hist, exists := a.histograms[name]
	if !exists {
		hist = NewHistogram()
		a.histograms[name] = hist
	}
	a.mu.Unlock()

	hist.Record(value)
}

// --- Request Tracking ---

// TrackRequest returns a function to call when request completes
// Usage: defer agent.TrackRequest()()
func (a *Agent) TrackRequest() func() {
	start := time.Now()
	a.inflight.Add(1)

	return func() {
		a.inflight.Add(-1)
		latency := float64(time.Since(start).Milliseconds())
		a.RecordHistogram("latency", latency)
	}
}

// RecordError records an error occurrence
func (a *Agent) RecordError(errorType string) {
	a.IncCounter("errors_" + errorType)
	a.IncCounter("errors_total")
}

// --- Helper Functions ---

func generateInstanceID() string {
	const charset = "abcdefghijklmnopqrstuvwxyz0123456789"
	b := make([]byte, 8)
	for i := range b {
		b[i] = charset[rand.Intn(len(charset))]
	}
	return string(b)
}
