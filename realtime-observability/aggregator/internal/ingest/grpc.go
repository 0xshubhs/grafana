package ingest

// This file contains the generated gRPC service interface
// In production, this would be generated from the protobuf file

// TelemetryIngestorServer is the server API for TelemetryIngestor service
type TelemetryIngestorServer interface {
	StreamTelemetry(TelemetryIngestor_StreamTelemetryServer) error
}

// UnimplementedTelemetryIngestorServer can be embedded to have forward compatible implementations
type UnimplementedTelemetryIngestorServer struct{}

func (UnimplementedTelemetryIngestorServer) StreamTelemetry(TelemetryIngestor_StreamTelemetryServer) error {
	return nil
}

// TelemetryIngestor_StreamTelemetryServer is the server-side stream interface
type TelemetryIngestor_StreamTelemetryServer interface {
	Send(*Ack) error
	SendAndClose(*Ack) error
	Recv() (*TelemetryBatch, error)
}

// RegisterTelemetryIngestorServer registers the server
func RegisterTelemetryIngestorServer(s interface{}, srv TelemetryIngestorServer) {
	// In production, this would use the generated code from protoc
}

// Message types (normally generated from proto)

type Ack struct {
	Ok bool
}

type TelemetryBatch struct {
	Service  string
	Instance string
	Metrics  []*Metric
}

type Metric struct {
	Name    string
	Labels  map[string]string
	Samples []*MetricSample
}

type MetricSample struct {
	TimestampNs uint64
	Value       isMetricSample_Value
}

type isMetricSample_Value interface {
	isMetricSample_Value()
}

type MetricSample_Gauge struct {
	Gauge float64
}

type MetricSample_Counter struct {
	Counter uint64
}

type MetricSample_Histogram struct {
	Histogram *Histogram
}

func (*MetricSample_Gauge) isMetricSample_Value()     {}
func (*MetricSample_Counter) isMetricSample_Value()   {}
func (*MetricSample_Histogram) isMetricSample_Value() {}

func (m *MetricSample) GetGauge() float64 {
	if v, ok := m.Value.(*MetricSample_Gauge); ok {
		return v.Gauge
	}
	return 0
}

func (m *MetricSample) GetCounter() uint64 {
	if v, ok := m.Value.(*MetricSample_Counter); ok {
		return v.Counter
	}
	return 0
}

func (m *MetricSample) GetHistogram() *Histogram {
	if v, ok := m.Value.(*MetricSample_Histogram); ok {
		return v.Histogram
	}
	return nil
}

type Histogram struct {
	Bounds []float64
	Counts []uint64
}
