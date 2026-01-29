import { useMetricsStore } from './store';

const WS_URL = import.meta.env.PROD 
  ? `ws://${window.location.host}/ws`
  : 'ws://localhost:8080/ws';

const RECONNECT_DELAY = 1000;
const MAX_RECONNECT_DELAY = 30000;

class WebSocketClient {
  private ws: WebSocket | null = null;
  private reconnectDelay = RECONNECT_DELAY;
  private reconnectTimeout: ReturnType<typeof setTimeout> | null = null;
  private subscriptions: Array<{ service: string; metric: string }> = [];

  connect() {
    if (this.ws?.readyState === WebSocket.OPEN) {
      return;
    }

    console.log('Connecting to WebSocket:', WS_URL);
    this.ws = new WebSocket(WS_URL);

    this.ws.onopen = () => {
      console.log('WebSocket connected');
      useMetricsStore.getState().setConnected(true);
      this.reconnectDelay = RECONNECT_DELAY;
      
      // Re-send subscriptions after reconnect
      if (this.subscriptions.length > 0) {
        this.subscribe(this.subscriptions);
      }
    };

    this.ws.onmessage = (event) => {
      try {
        // Handle batched messages (newline separated)
        const messages = event.data.split('\n');
        for (const msg of messages) {
          if (msg.trim()) {
            const data = JSON.parse(msg);
            if (data.type === 'snapshot') {
              useMetricsStore.getState().update(data);
            }
          }
        }
      } catch (error) {
        console.error('Failed to parse message:', error);
      }
    };

    this.ws.onclose = (event) => {
      console.log('WebSocket closed:', event.code, event.reason);
      useMetricsStore.getState().setConnected(false);
      this.scheduleReconnect();
    };

    this.ws.onerror = (error) => {
      console.error('WebSocket error:', error);
    };
  }

  private scheduleReconnect() {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
    }

    console.log(`Reconnecting in ${this.reconnectDelay}ms...`);
    this.reconnectTimeout = setTimeout(() => {
      this.connect();
    }, this.reconnectDelay);

    // Exponential backoff
    this.reconnectDelay = Math.min(this.reconnectDelay * 2, MAX_RECONNECT_DELAY);
  }

  subscribe(subscriptions: Array<{ service: string; metric: string }>) {
    this.subscriptions = subscriptions;
    
    if (this.ws?.readyState === WebSocket.OPEN) {
      this.ws.send(JSON.stringify({
        type: 'subscribe',
        subscriptions,
      }));
    }
  }

  disconnect() {
    if (this.reconnectTimeout) {
      clearTimeout(this.reconnectTimeout);
    }
    this.ws?.close();
  }
}

// Singleton instance
export const wsClient = new WebSocketClient();

// React hook for WebSocket connection
export function useWebSocket() {
  const connected = useMetricsStore((state) => state.connected);
  const lastUpdate = useMetricsStore((state) => state.lastUpdate);

  return {
    connected,
    lastUpdate,
    connect: () => wsClient.connect(),
    disconnect: () => wsClient.disconnect(),
    subscribe: (subs: Array<{ service: string; metric: string }>) => 
      wsClient.subscribe(subs),
  };
}
