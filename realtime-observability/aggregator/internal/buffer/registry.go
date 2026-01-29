package buffer

import (
	"fmt"
	"sync"
)

const (
	// DefaultRingSize is the default size for ring buffers (10 seconds at 100Hz)
	DefaultRingSize = 1000

	// HistogramRingSize for histogram data (less frequent)
	HistogramRingSize = 500
)

// MetricKey uniquely identifies a metric
type MetricKey struct {
	Service string
	Name    string
}

func (k MetricKey) String() string {
	return fmt.Sprintf("%s/%s", k.Service, k.Name)
}

// HistogramData holds histogram bounds and counts
type HistogramData struct {
	Ts     int64
	Bounds []float64
	Counts []uint64
}

// HistogramRing is a ring buffer for histogram samples
type HistogramRing struct {
	data []HistogramData
	idx  uint64
	size uint64
	mu   sync.RWMutex
}

// NewHistogramRing creates a new histogram ring buffer
func NewHistogramRing(size int) *HistogramRing {
	return &HistogramRing{
		data: make([]HistogramData, size),
		size: uint64(size),
	}
}

// Push adds a histogram sample
func (r *HistogramRing) Push(h HistogramData) {
	r.mu.Lock()
	r.data[r.idx%r.size] = h
	r.idx++
	r.mu.Unlock()
}

// Latest returns the most recent histogram
func (r *HistogramRing) Latest() (HistogramData, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if r.idx == 0 {
		return HistogramData{}, false
	}
	return r.data[(r.idx-1)%r.size], true
}

// Registry manages all metric ring buffers
type Registry struct {
	gauges     map[MetricKey]*Ring
	counters   map[MetricKey]*Ring
	histograms map[MetricKey]*HistogramRing
	mu         sync.RWMutex
}

// NewRegistry creates a new metric registry
func NewRegistry() *Registry {
	return &Registry{
		gauges:     make(map[MetricKey]*Ring),
		counters:   make(map[MetricKey]*Ring),
		histograms: make(map[MetricKey]*HistogramRing),
	}
}

// GetRing returns the ring buffer for a gauge metric, creating if needed
func (r *Registry) GetRing(service, name string) *Ring {
	key := MetricKey{Service: service, Name: name}

	r.mu.RLock()
	ring, exists := r.gauges[key]
	r.mu.RUnlock()

	if exists {
		return ring
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	// Double-check after acquiring write lock
	if ring, exists = r.gauges[key]; exists {
		return ring
	}

	ring = NewRing(DefaultRingSize)
	r.gauges[key] = ring
	return ring
}

// GetCounterRing returns the ring buffer for a counter metric
func (r *Registry) GetCounterRing(service, name string) *Ring {
	key := MetricKey{Service: service, Name: name}

	r.mu.RLock()
	ring, exists := r.counters[key]
	r.mu.RUnlock()

	if exists {
		return ring
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if ring, exists = r.counters[key]; exists {
		return ring
	}

	ring = NewRing(DefaultRingSize)
	r.counters[key] = ring
	return ring
}

// GetHistogramRing returns the ring buffer for a histogram metric
func (r *Registry) GetHistogramRing(service, name string) *HistogramRing {
	key := MetricKey{Service: service, Name: name}

	r.mu.RLock()
	ring, exists := r.histograms[key]
	r.mu.RUnlock()

	if exists {
		return ring
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	if ring, exists = r.histograms[key]; exists {
		return ring
	}

	ring = NewHistogramRing(HistogramRingSize)
	r.histograms[key] = ring
	return ring
}

// Snapshot returns all current metrics data
type MetricsSnapshot struct {
	Gauges     map[MetricKey][]Sample
	Counters   map[MetricKey][]Sample
	Histograms map[MetricKey]HistogramData
}

// Snapshot returns a point-in-time snapshot of all metrics
func (r *Registry) Snapshot() MetricsSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := MetricsSnapshot{
		Gauges:     make(map[MetricKey][]Sample),
		Counters:   make(map[MetricKey][]Sample),
		Histograms: make(map[MetricKey]HistogramData),
	}

	for key, ring := range r.gauges {
		snapshot.Gauges[key] = ring.SnapshotLast(100) // Last 100 samples
	}

	for key, ring := range r.counters {
		snapshot.Counters[key] = ring.SnapshotLast(100)
	}

	for key, ring := range r.histograms {
		if h, ok := ring.Latest(); ok {
			snapshot.Histograms[key] = h
		}
	}

	return snapshot
}

// LatestSnapshot returns only the latest value for each metric
type LatestSnapshot struct {
	Gauges     map[MetricKey]Sample
	Counters   map[MetricKey]Sample
	Histograms map[MetricKey]HistogramData
}

// LatestSnapshot returns the most recent value for all metrics
func (r *Registry) LatestSnapshot() LatestSnapshot {
	r.mu.RLock()
	defer r.mu.RUnlock()

	snapshot := LatestSnapshot{
		Gauges:     make(map[MetricKey]Sample),
		Counters:   make(map[MetricKey]Sample),
		Histograms: make(map[MetricKey]HistogramData),
	}

	for key, ring := range r.gauges {
		if s, ok := ring.Latest(); ok {
			snapshot.Gauges[key] = s
		}
	}

	for key, ring := range r.counters {
		if s, ok := ring.Latest(); ok {
			snapshot.Counters[key] = s
		}
	}

	for key, ring := range r.histograms {
		if h, ok := ring.Latest(); ok {
			snapshot.Histograms[key] = h
		}
	}

	return snapshot
}

// ListServices returns all registered services
func (r *Registry) ListServices() []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	services := make(map[string]struct{})
	for key := range r.gauges {
		services[key.Service] = struct{}{}
	}
	for key := range r.counters {
		services[key.Service] = struct{}{}
	}
	for key := range r.histograms {
		services[key.Service] = struct{}{}
	}

	result := make([]string, 0, len(services))
	for s := range services {
		result = append(result, s)
	}
	return result
}

// ListMetrics returns all metric names for a service
func (r *Registry) ListMetrics(service string) []string {
	r.mu.RLock()
	defer r.mu.RUnlock()

	metrics := make(map[string]struct{})
	for key := range r.gauges {
		if key.Service == service {
			metrics[key.Name] = struct{}{}
		}
	}
	for key := range r.counters {
		if key.Service == service {
			metrics[key.Name] = struct{}{}
		}
	}
	for key := range r.histograms {
		if key.Service == service {
			metrics[key.Name] = struct{}{}
		}
	}

	result := make([]string, 0, len(metrics))
	for m := range metrics {
		result = append(result, m)
	}
	return result
}
