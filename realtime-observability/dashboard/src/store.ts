import { create } from 'zustand';

// Types
export interface Sample {
  ts: number;
  val: number;
}

export interface HistogramData {
  ts: number;
  bounds: number[];
  counts: number[];
}

export interface MetricData {
  gauges: Map<string, Sample>;
  counters: Map<string, Sample>;
  histograms: Map<string, HistogramData>;
}

export interface TimeSeriesPoint {
  time: number;
  value: number;
}

interface MetricsStore {
  // Current values
  metrics: MetricData;
  
  // Time series buffers (fixed window)
  timeSeries: Map<string, Float32Array>;
  timeSeriesIndex: Map<string, number>;
  
  // Connection state
  connected: boolean;
  lastUpdate: number;
  
  // Actions
  update: (data: any) => void;
  setConnected: (connected: boolean) => void;
  getTimeSeries: (key: string, count: number) => TimeSeriesPoint[];
  getLatestValue: (key: string) => number | null;
  getHistogram: (key: string) => HistogramData | null;
}

const BUFFER_SIZE = 500; // ~10 seconds at 50Hz

// Pre-allocate buffers
const createBuffer = () => new Float32Array(BUFFER_SIZE);

export const useMetricsStore = create<MetricsStore>((set, get) => ({
  metrics: {
    gauges: new Map(),
    counters: new Map(),
    histograms: new Map(),
  },
  
  timeSeries: new Map(),
  timeSeriesIndex: new Map(),
  
  connected: false,
  lastUpdate: 0,
  
  update: (data: any) => {
    const state = get();
    const now = Date.now();
    
    // Update gauges
    if (data.gauges) {
      for (const [key, value] of Object.entries(data.gauges)) {
        const sample = value as Sample;
        state.metrics.gauges.set(key, sample);
        
        // Update time series buffer
        let buffer = state.timeSeries.get(key);
        let index = state.timeSeriesIndex.get(key) || 0;
        
        if (!buffer) {
          buffer = createBuffer();
          state.timeSeries.set(key, buffer);
        }
        
        buffer[index % BUFFER_SIZE] = sample.val;
        state.timeSeriesIndex.set(key, index + 1);
      }
    }
    
    // Update counters
    if (data.counters) {
      for (const [key, value] of Object.entries(data.counters)) {
        state.metrics.counters.set(key, value as Sample);
      }
    }
    
    // Update histograms
    if (data.histograms) {
      for (const [key, value] of Object.entries(data.histograms)) {
        state.metrics.histograms.set(key, value as HistogramData);
      }
    }
    
    set({ lastUpdate: now });
  },
  
  setConnected: (connected: boolean) => set({ connected }),
  
  getTimeSeries: (key: string, count: number): TimeSeriesPoint[] => {
    const state = get();
    const buffer = state.timeSeries.get(key);
    const index = state.timeSeriesIndex.get(key) || 0;
    
    if (!buffer || index === 0) return [];
    
    const result: TimeSeriesPoint[] = [];
    const start = Math.max(0, index - count);
    const now = Date.now();
    
    for (let i = start; i < index; i++) {
      result.push({
        time: now - (index - i) * 20, // Approximate 20ms intervals
        value: buffer[i % BUFFER_SIZE],
      });
    }
    
    return result;
  },
  
  getLatestValue: (key: string): number | null => {
    const state = get();
    const gauge = state.metrics.gauges.get(key);
    if (gauge) return gauge.val;
    
    const counter = state.metrics.counters.get(key);
    if (counter) return counter.val;
    
    return null;
  },
  
  getHistogram: (key: string): HistogramData | null => {
    const state = get();
    return state.metrics.histograms.get(key) || null;
  },
}));

// Derived selectors
export const selectServices = (state: MetricsStore): string[] => {
  const services = new Set<string>();
  for (const key of state.metrics.gauges.keys()) {
    const service = key.split('/')[0];
    if (service) services.add(service);
  }
  return Array.from(services);
};

export const selectMetricsForService = (state: MetricsStore, service: string) => {
  const metrics: string[] = [];
  for (const key of state.metrics.gauges.keys()) {
    if (key.startsWith(`${service}/`)) {
      metrics.push(key.split('/')[1]);
    }
  }
  return metrics;
};
