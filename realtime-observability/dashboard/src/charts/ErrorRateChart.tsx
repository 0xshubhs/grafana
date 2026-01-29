import { useMemo } from 'react';
import {
  AreaChart,
  Area,
  XAxis,
  YAxis,
  CartesianGrid,
  Tooltip,
  ResponsiveContainer,
} from 'recharts';
import { useMetricsStore } from '../store';

interface ErrorRateChartProps {
  service: string;
  height?: number;
  windowSeconds?: number;
}

export function ErrorRateChart({ 
  service, 
  height = 200, 
  windowSeconds = 10 
}: ErrorRateChartProps) {
  const getTimeSeries = useMetricsStore((state) => state.getTimeSeries);
  const lastUpdate = useMetricsStore((state) => state.lastUpdate);

  const data = useMemo(() => {
    const errorRate = getTimeSeries(`${service}/error_rate`, windowSeconds * 50);
    return errorRate.slice(-250).map((p) => ({
      time: p.time,
      rate: p.value * 100, // Convert to percentage
    }));
  }, [service, getTimeSeries, lastUpdate, windowSeconds]);

  const formatTime = (time: number) => {
    const date = new Date(time);
    return `${date.getMinutes()}:${date.getSeconds().toString().padStart(2, '0')}`;
  };

  return (
    <div className="chart-container">
      <h3 className="text-lg font-semibold mb-2">Error Rate - {service}</h3>
      <ResponsiveContainer width="100%" height={height}>
        <AreaChart data={data} margin={{ top: 5, right: 20, left: 0, bottom: 5 }}>
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
            tickFormatter={(v) => `${v}%`}
            domain={[0, 'auto']}
          />
          <Tooltip
            contentStyle={{
              backgroundColor: '#1e293b',
              border: '1px solid #334155',
              borderRadius: '4px',
            }}
            labelFormatter={(time) => new Date(time).toLocaleTimeString()}
            formatter={(value: number) => [`${value.toFixed(2)}%`]}
          />
          <defs>
            <linearGradient id="errorGradient" x1="0" y1="0" x2="0" y2="1">
              <stop offset="5%" stopColor="#ef4444" stopOpacity={0.4} />
              <stop offset="95%" stopColor="#ef4444" stopOpacity={0} />
            </linearGradient>
          </defs>
          <Area
            type="monotone"
            dataKey="rate"
            stroke="#ef4444"
            fill="url(#errorGradient)"
            strokeWidth={2}
          />
        </AreaChart>
      </ResponsiveContainer>
    </div>
  );
}
