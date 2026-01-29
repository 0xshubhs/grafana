pub mod telemetry {
    tonic::include_proto!("telemetry");
}

use parking_lot::Mutex;
use std::collections::HashMap;
use std::sync::atomic::{AtomicI64, AtomicU64, Ordering};
use std::sync::Arc;
use std::time::{Duration, Instant, SystemTime, UNIX_EPOCH};
use tokio::sync::mpsc;
use tokio::time::interval;
use tonic::transport::Channel;

use telemetry::telemetry_ingestor_client::TelemetryIngestorClient;
use telemetry::{Histogram as HistogramProto, Metric, MetricSample, TelemetryBatch};

/// Default histogram bounds for latency tracking (in milliseconds)
const DEFAULT_BOUNDS: [f64; 12] = [
    1.0, 5.0, 10.0, 25.0, 50.0, 100.0, 250.0, 500.0, 1000.0, 2500.0, 5000.0, 10000.0,
];

/// Lock-free histogram for latency tracking
pub struct Histogram {
    bounds: Vec<f64>,
    counts: Vec<AtomicU64>,
}

impl Histogram {
    pub fn new() -> Self {
        Self {
            bounds: DEFAULT_BOUNDS.to_vec(),
            counts: (0..DEFAULT_BOUNDS.len() + 1)
                .map(|_| AtomicU64::new(0))
                .collect(),
        }
    }

    pub fn record(&self, value: f64) {
        for (i, bound) in self.bounds.iter().enumerate() {
            if value <= *bound {
                self.counts[i].fetch_add(1, Ordering::Relaxed);
                return;
            }
        }
        self.counts[self.counts.len() - 1].fetch_add(1, Ordering::Relaxed);
    }

    pub fn snapshot_and_reset(&self) -> (Vec<f64>, Vec<u64>) {
        let counts: Vec<u64> = self
            .counts
            .iter()
            .map(|c| c.swap(0, Ordering::Relaxed))
            .collect();
        (self.bounds.clone(), counts)
    }
}

impl Default for Histogram {
    fn default() -> Self {
        Self::new()
    }
}

/// Agent configuration
#[derive(Clone)]
pub struct Config {
    pub aggregator_addr: String,
    pub service_name: String,
    pub instance_id: String,
    pub push_interval: Duration,
}

impl Default for Config {
    fn default() -> Self {
        Self {
            aggregator_addr: "http://localhost:9000".to_string(),
            service_name: "default".to_string(),
            instance_id: generate_instance_id(),
            push_interval: Duration::from_millis(20),
        }
    }
}

/// Telemetry agent for collecting and pushing metrics
pub struct Agent {
    config: Config,
    gauges: Arc<Mutex<HashMap<String, f64>>>,
    counters: Arc<Mutex<HashMap<String, AtomicU64>>>,
    histograms: Arc<Mutex<HashMap<String, Arc<Histogram>>>>,
    inflight: Arc<AtomicI64>,
    shutdown_tx: Option<mpsc::Sender<()>>,
}

impl Agent {
    pub fn new(config: Config) -> Self {
        Self {
            config,
            gauges: Arc::new(Mutex::new(HashMap::new())),
            counters: Arc::new(Mutex::new(HashMap::new())),
            histograms: Arc::new(Mutex::new(HashMap::new())),
            inflight: Arc::new(AtomicI64::new(0)),
            shutdown_tx: None,
        }
    }

    /// Connect and start the agent
    pub async fn start(&mut self) -> Result<(), Box<dyn std::error::Error>> {
        let channel = Channel::from_shared(self.config.aggregator_addr.clone())?
            .connect()
            .await?;

        let client = TelemetryIngestorClient::new(channel);
        let (shutdown_tx, mut shutdown_rx) = mpsc::channel(1);
        self.shutdown_tx = Some(shutdown_tx);

        let config = self.config.clone();
        let gauges = self.gauges.clone();
        let counters = self.counters.clone();
        let histograms = self.histograms.clone();
        let inflight = self.inflight.clone();

        tokio::spawn(async move {
            let mut interval = interval(config.push_interval);
            let mut client = client;

            loop {
                tokio::select! {
                    _ = interval.tick() => {
                        let batch = collect_metrics(
                            &config,
                            &gauges,
                            &counters,
                            &histograms,
                            &inflight,
                        );

                        if !batch.metrics.is_empty() {
                            let stream = async_stream::stream! {
                                yield batch;
                            };

                            if let Err(e) = client.stream_telemetry(stream).await {
                                eprintln!("Failed to send metrics: {}", e);
                            }
                        }
                    }
                    _ = shutdown_rx.recv() => {
                        break;
                    }
                }
            }
        });

        Ok(())
    }

    /// Stop the agent
    pub async fn stop(&mut self) {
        if let Some(tx) = self.shutdown_tx.take() {
            let _ = tx.send(()).await;
        }
    }

    /// Set a gauge metric value
    pub fn set_gauge(&self, name: &str, value: f64) {
        let mut gauges = self.gauges.lock();
        gauges.insert(name.to_string(), value);
    }

    /// Increment a counter
    pub fn inc_counter(&self, name: &str) {
        let mut counters = self.counters.lock();
        counters
            .entry(name.to_string())
            .or_insert_with(|| AtomicU64::new(0))
            .fetch_add(1, Ordering::Relaxed);
    }

    /// Record a histogram value
    pub fn record_histogram(&self, name: &str, value: f64) {
        let hist = {
            let mut histograms = self.histograms.lock();
            histograms
                .entry(name.to_string())
                .or_insert_with(|| Arc::new(Histogram::new()))
                .clone()
        };
        hist.record(value);
    }

    /// Track a request (returns guard that records latency on drop)
    pub fn track_request(&self) -> RequestGuard {
        self.inflight.fetch_add(1, Ordering::Relaxed);
        RequestGuard {
            start: Instant::now(),
            inflight: self.inflight.clone(),
            histograms: self.histograms.clone(),
        }
    }

    /// Record an error
    pub fn record_error(&self, error_type: &str) {
        self.inc_counter(&format!("errors_{}", error_type));
        self.inc_counter("errors_total");
    }
}

/// Guard that records latency when dropped
pub struct RequestGuard {
    start: Instant,
    inflight: Arc<AtomicI64>,
    histograms: Arc<Mutex<HashMap<String, Arc<Histogram>>>>,
}

impl Drop for RequestGuard {
    fn drop(&mut self) {
        self.inflight.fetch_sub(1, Ordering::Relaxed);
        let latency = self.start.elapsed().as_secs_f64() * 1000.0;

        let hist = {
            let mut histograms = self.histograms.lock();
            histograms
                .entry("latency".to_string())
                .or_insert_with(|| Arc::new(Histogram::new()))
                .clone()
        };
        hist.record(latency);
    }
}

fn collect_metrics(
    config: &Config,
    gauges: &Arc<Mutex<HashMap<String, f64>>>,
    counters: &Arc<Mutex<HashMap<String, AtomicU64>>>,
    histograms: &Arc<Mutex<HashMap<String, Arc<Histogram>>>>,
    inflight: &Arc<AtomicI64>,
) -> TelemetryBatch {
    let now = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos() as u64;

    let mut metrics = Vec::new();

    // Collect gauges
    {
        let gauges = gauges.lock();
        for (name, value) in gauges.iter() {
            metrics.push(Metric {
                name: name.clone(),
                labels: HashMap::new(),
                samples: vec![MetricSample {
                    timestamp_ns: now,
                    value: Some(telemetry::metric_sample::Value::Gauge(*value)),
                }],
            });
        }
    }

    // Collect counters
    {
        let counters = counters.lock();
        for (name, counter) in counters.iter() {
            metrics.push(Metric {
                name: name.clone(),
                labels: HashMap::new(),
                samples: vec![MetricSample {
                    timestamp_ns: now,
                    value: Some(telemetry::metric_sample::Value::Counter(
                        counter.load(Ordering::Relaxed),
                    )),
                }],
            });
        }
    }

    // Collect histograms
    {
        let histograms = histograms.lock();
        for (name, hist) in histograms.iter() {
            let (bounds, counts) = hist.snapshot_and_reset();
            metrics.push(Metric {
                name: name.clone(),
                labels: HashMap::new(),
                samples: vec![MetricSample {
                    timestamp_ns: now,
                    value: Some(telemetry::metric_sample::Value::Histogram(HistogramProto {
                        bounds,
                        counts,
                    })),
                }],
            });
        }
    }

    // Add inflight gauge
    metrics.push(Metric {
        name: "inflight".to_string(),
        labels: HashMap::new(),
        samples: vec![MetricSample {
            timestamp_ns: now,
            value: Some(telemetry::metric_sample::Value::Gauge(
                inflight.load(Ordering::Relaxed) as f64,
            )),
        }],
    });

    TelemetryBatch {
        service: config.service_name.clone(),
        instance: config.instance_id.clone(),
        metrics,
    }
}

fn generate_instance_id() -> String {
    use std::time::SystemTime;
    let nanos = SystemTime::now()
        .duration_since(UNIX_EPOCH)
        .unwrap()
        .as_nanos();
    format!("{:x}", nanos % 0xFFFFFFFF)
}

#[cfg(test)]
mod tests {
    use super::*;

    #[test]
    fn test_histogram() {
        let hist = Histogram::new();
        hist.record(5.0);
        hist.record(50.0);
        hist.record(500.0);

        let (bounds, counts) = hist.snapshot_and_reset();
        assert!(!bounds.is_empty());
        assert!(counts.iter().sum::<u64>() == 3);
    }
}
