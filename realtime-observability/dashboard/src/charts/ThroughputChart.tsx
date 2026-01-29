import { useMemo } from 'react';
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { useMetricsStore } from '../store';

interface ThroughputChartProps {
  service: string;
  height?: number;
  windowSeconds?: number;
}

export function ThroughputChart({ 
  service, 
  height = 200, 
  windowSeconds = 10 
}: ThroughputChartProps) {
  const getTimeSeries = useMetricsStore((state) => state.getTimeSeries);
  const lastUpdate = useMetricsStore((state) => state.lastUpdate);

  const data = useMemo(() => {
    const rps = getTimeSeries(`${service}/rps`, windowSeconds * 50);
    return rps.slice(-250).map((p) => ({
      time: p.time,
      rps: p.value,
    }));
  }, [service, getTimeSeries, lastUpdate, windowSeconds]);

  const formatTime = (time: number) => {
    const date = new Date(time);
    return `${date.getMinutes()}:${date.getSeconds().toString().padStart(2, '0')}`;
  };

  return (
    <div className="chart-container">
      <h3 className="text-lg font-semibold mb-2">Throughput - {service}</h3>
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
            tickFormatter={(v) => `${v}/s`}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: '#1e293b',
              border: '1px solid #334155',
              borderRadius: '4px',
            }}
            labelFormatter={(time) => new Date(time).toLocaleTimeString()}
            formatter={(value: number) => [`${value.toFixed(0)} req/s`]}
          />
          <Line
            type="monotone"
            dataKey="rps"
            stroke="#3b82f6"
            dot={false}
            strokeWidth={2}
            fill="url(#rpsGradient)"
          />
          <defs>
            <linearGradient id="rpsGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#3b82f6" stopOpacity={0.3} />
              <stop offset="95%" stopColor="#3b82f6" stopOpacity={0} />
            </linearGradient>
          </defs>
        </LineChart>
      </ResponsiveContainer>
    </div>
  );
}
