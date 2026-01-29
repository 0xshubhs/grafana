import { useMemo } from 'react';
import {
  BarChart,
  Bar,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
  Cell,
} from 'recharts';
import { useMetricsStore, HistogramData } from '../store';

interface HistogramChartProps {
  service: string;
  metric: string;
  height?: number;
}

export function HistogramChart({ 
  service, 
  metric, 
  height = 200 
}: HistogramChartProps) {
  const getHistogram = useMetricsStore((state) => state.getHistogram);
  const lastUpdate = useMetricsStore((state) => state.lastUpdate);

  const data = useMemo(() => {
    const hist = getHistogram(`${service}/${metric}`);
    if (!hist || !hist.bounds || !hist.counts) return [];

    return hist.bounds.map((bound, i) => ({
      bucket: `≤${bound}ms`,
      count: hist.counts[i] || 0,
      bound,
    }));
  }, [service, metric, getHistogram, lastUpdate]);

  // Color based on latency threshold
  const getBarColor = (bound: number) => {
    if (bound <= 50) return '#10b981';
    if (bound <= 200) return '#f59e0b';
    return '#ef4444';
  };

  return (
    <div className="chart-container">
      <h3 className="text-lg font-semibold mb-2">
        {metric} Distribution - {service}
      </h3>
      <ResponsiveContainer width="100%" height={height}>
        <BarChart data={data} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
          <CartesianGrid strokeDasharray="3 3" stroke="#334155" />
          <XAxis
            dataKey="bucket"
            stroke="#94a3b8"
            tick={{ fontSize: 10 }}
            angle={-45}
            textAnchor="end"
            height={60}
          />
          <YAxis stroke="#94a3b8" tick={{ fontSize: 12 }} />
          <Tooltip
            contentStyle={{
              backgroundColor: '#1e293b',
              border: '1px solid #334155',
              borderRadius: '4px',
            }}
            formatter={(value: number) => [`${value} requests`]}
          />
          <Bar dataKey="count" radius={[4, 4, 0, 0]}>
            {data.map((entry, index) => (
              <Cell key={`cell-${index}`} fill={getBarColor(entry.bound)} />
            ))}
          </Bar>
        </BarChart>
      </ResponsiveContainer>
    </div>
  );
}
