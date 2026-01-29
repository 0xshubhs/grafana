import { useEffect, useState } from 'react';
import { useWebSocket, wsClient } from './ws';
import { useMetricsStore, selectServices } from './store';
import { LatencyChart, ThroughputChart, ErrorRateChart, HistogramChart } from './charts';

function ConnectionStatus() {
  const { connected, lastUpdate } = useWebSocket();
  
  return (
    <div className="flex items-center gap-2">
      <div 
        className={`w-3 h-3 rounded-full ${
          connected ? 'bg-green-500 animate-pulse-glow' : 'bg-red-500'
        }`}
      />
      <span className="text-sm text-gray-400">
        {connected ? 'Connected' : 'Disconnected'}
      </span>
      {lastUpdate > 0 && (
        <span className="text-xs text-gray-500">
          Last: {new Date(lastUpdate).toLocaleTimeString()}
        </span>
      )}
    </div>
  );
}

function MetricCard({ label, value, unit, status }: {
  label: string;
  value: number | string;
  unit?: string;
  status?: 'healthy' | 'warning' | 'critical';
}) {
  const statusColors = {
    healthy: 'text-green-400',
    warning: 'text-yellow-400',
    critical: 'text-red-400',
  };

  return (
    <div className="metric-card">
      <div className="text-sm text-gray-400 mb-1">{label}</div>
      <div className={`text-2xl font-bold ${status ? statusColors[status] : 'text-white'}`}>
        {typeof value === 'number' ? value.toFixed(2) : value}
        {unit && <span className="text-sm ml-1 text-gray-400">{unit}</span>}
      </div>
    </div>
  );
}

function ServiceDashboard({ service }: { service: string }) {
  const getLatestValue = useMetricsStore((state) => state.getLatestValue);
  const lastUpdate = useMetricsStore((state) => state.lastUpdate);

  const p50 = getLatestValue(`${service}/latency_p50`) || 0;
  const p99 = getLatestValue(`${service}/latency_p99`) || 0;
  const rps = getLatestValue(`${service}/rps`) || 0;
  const errorRate = (getLatestValue(`${service}/error_rate`) || 0) * 100;
  const inflight = getLatestValue(`${service}/inflight`) || 0;

  const getLatencyStatus = (val: number): 'healthy' | 'warning' | 'critical' => {
    if (val < 50) return 'healthy';
    if (val < 200) return 'warning';
    return 'critical';
  };

  const getErrorStatus = (val: number): 'healthy' | 'warning' | 'critical' => {
    if (val < 1) return 'healthy';
    if (val < 5) return 'warning';
    return 'critical';
  };

  return (
    <div className="space-y-4">
      <h2 className="text-xl font-bold text-white border-b border-dark-100 pb-2">
        {service}
      </h2>
      
      {/* Key metrics */}
      <div className="grid grid-cols-2 md:grid-cols-5 gap-4">
        <MetricCard 
          label="P50 Latency" 
          value={p50} 
          unit="ms" 
          status={getLatencyStatus(p50)}
        />
        <MetricCard 
          label="P99 Latency" 
          value={p99} 
          unit="ms" 
          status={getLatencyStatus(p99)}
        />
        <MetricCard 
          label="Throughput" 
          value={rps} 
          unit="req/s" 
        />
        <MetricCard 
          label="Error Rate" 
          value={errorRate} 
          unit="%" 
          status={getErrorStatus(errorRate)}
        />
        <MetricCard 
          label="In-flight" 
          value={inflight.toFixed(0)} 
        />
      </div>

      {/* Charts */}
      <div className="grid grid-cols-1 lg:grid-cols-2 gap-4">
        <LatencyChart service={service} height={250} />
        <ThroughputChart service={service} height={250} />
        <ErrorRateChart service={service} height={250} />
        <HistogramChart service={service} metric="latency" height={250} />
      </div>
    </div>
  );
}

export default function App() {
  const services = useMetricsStore(selectServices);
  const [selectedService, setSelectedService] = useState<string | null>(null);

  // Connect WebSocket on mount
  useEffect(() => {
    wsClient.connect();
    return () => wsClient.disconnect();
  }, []);

  // Select first service when available
  useEffect(() => {
    if (services.length > 0 && !selectedService) {
      setSelectedService(services[0]);
    }
  }, [services, selectedService]);

  return (
    <div className="min-h-screen bg-dark-300">
      {/* Header */}
      <header className="bg-dark-200 border-b border-dark-100 px-6 py-4">
        <div className="flex items-center justify-between">
          <div className="flex items-center gap-4">
            <h1 className="text-2xl font-bold text-white">
              📊 Real-time Observability
            </h1>
            <ConnectionStatus />
          </div>
          
          {/* Service selector */}
          {services.length > 0 && (
            <div className="flex items-center gap-2">
              <label className="text-sm text-gray-400">Service:</label>
              <select
                value={selectedService || ''}
                onChange={(e) => setSelectedService(e.target.value)}
                className="bg-dark-100 text-white border border-dark-100 rounded px-3 py-1"
              >
                {services.map((s) => (
                  <option key={s} value={s}>{s}</option>
                ))}
              </select>
            </div>
          )}
        </div>
      </header>

      {/* Main content */}
      <main className="p-6">
        {selectedService ? (
          <ServiceDashboard service={selectedService} />
        ) : (
          <div className="flex items-center justify-center h-64">
            <div className="text-center">
              <div className="text-6xl mb-4">📡</div>
              <p className="text-gray-400">
                Waiting for telemetry data...
              </p>
              <p className="text-sm text-gray-500 mt-2">
                Make sure the aggregator is running and agents are connected.
              </p>
            </div>
          </div>
        )}
      </main>

      {/* Footer */}
      <footer className="fixed bottom-0 left-0 right-0 bg-dark-200 border-t border-dark-100 px-6 py-2">
        <div className="flex items-center justify-between text-xs text-gray-500">
          <span>Real-time Observability Platform</span>
          <span>
            {services.length} service{services.length !== 1 ? 's' : ''} connected
          </span>
        </div>
      </footer>
    </div>
  );
}
