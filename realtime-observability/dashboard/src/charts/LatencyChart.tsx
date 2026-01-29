import { useEffect, useMemo } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Legend,
} from 'recharts';
import { useMetricsStore, TimeSeriesPoint } from '../store';

interface LatencyChartProps {
  service: string;
  height?: number;
  windowSeconds?: number;
}

export function LatencyChart({ 
  service, 
  height = 200, 
  windowSeconds = 10 
}: LatencyChartProps) {
  const getTimeSeries = useMetricsStore((state) => state.getTimeSeries);
  const lastUpdate = useMetricsStore((state) => state.lastUpdate);

  // Get data for p50, p95, p99
  const data = useMemo(() => {
    const p50 = getTimeSeries(`${service}/latency_p50`, windowSeconds * 50);
    const p95 = getTimeSeries(`${service}/latency_p95`, windowSeconds * 50);
    const p99 = getTimeSeries(`${service}/latency_p99`, windowSeconds * 50);

    // Merge time series
    const merged: Array<{
      time: number;
      p50?: number;
      p95?: number;
      p99?: number;
    }> = [];

    const timeSet = new Set<number>();
    [...p50, ...p95, ...p99].forEach((p) => timeSet.add(p.time));

    const times = Array.from(timeSet).sort((a, b) => a - b);
    
    const p50Map = new Map(p50.map((p) => [p.time, p.value]));
    const p95Map = new Map(p95.map((p) => [p.time, p.value]));
    const p99Map = new Map(p99.map((p) => [p.time, p.value]));

    for (const time of times) {
      merged.push({
        time,
        p50: p50Map.get(time),
        p95: p95Map.get(time),
        p99: p99Map.get(time),
      });
    }

    // Keep only last N points for performance
    return merged.slice(-250);
  }, [service, getTimeSeries, lastUpdate, windowSeconds]);

  const formatTime = (time: number) => {
    const date = new Date(time);
    return `${date.getMinutes()}:${date.getSeconds().toString().padStart(2, '0')}`;
  };

  return (
    <div className="chart-container">
      <h3 className="text-lg font-semibold mb-2">Latency - {service}</h3>
      <ResponsiveContainer width="100%" height={height}>
        <LineChart data={data} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis
            dataKey="time"
            tickFormatter={formatTime}
            stroke="#94a3b8"
            tick={{ fontSize: 12 }}
          />
          <YAxis
            stroke="#94a3b8"
            tick={{ fontSize: 12 }}
            tickFormatter={(v) => `${v}ms`}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: '#1e293b',
              border: '1px solid #334155',
              borderRadius: '4px',
            }}
            labelFormatter={(time) => new Date(time).toLocaleTimeString()}
            formatter={(value: number) => [`${value.toFixed(2)}ms`]}
          />
          <Legend />
          <Line
            type="monotone"
            dataKey="p50"
            stroke="#10b981"
            dot={false}
            strokeWidth={2}
            name="P50"
          />
          <Line
            type="monotone"
            dataKey="p95"
            stroke="#f59e0b"
            dot={false}
            strokeWidth={2}
            name="P95"
          />
          <Line
            type="monotone"
            dataKey="p99"
            stroke="#ef4444"
            dot={false}
            strokeWidth={2}
            name="P99"
          />
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
