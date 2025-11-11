import { useEffect, useRef, useCallback, useState } from "react";
import { useContainerStore } from "../store/containerStore";
import type {
  ContainerEventData,
  MetricsEventData,
} from "../lib/types";
import { websocketEventSchema } from "../lib/schemas";

const WS_URL =
  import.meta.env.MODE === "production"
    ? `${window.location.protocol === "https:" ? "wss:" : "ws:"}//${window.location.host}/ws/containers`
    : "ws://localhost:3000/ws/containers";

const RECONNECT_INTERVAL = 3000; // 3 seconds
const MAX_RECONNECT_ATTEMPTS = 10;

export function useWebSocket() {
  const ws = useRef<WebSocket | null>(null);
  const reconnectTimeoutRef = useRef<number | null>(null);
  const reconnectAttemptsRef = useRef(0);
  const [error, setError] = useState<string | null>(null);

  const setConnected = useContainerStore((state) => state.setConnected);
  const updateContainer = useContainerStore((state) => state.updateContainer);
  const removeContainer = useContainerStore((state) => state.removeContainer);
  const updateMetrics = useContainerStore((state) => state.updateMetrics);

  const handleMessage = useCallback(
    (event: MessageEvent) => {
      try {
        const data = JSON.parse(event.data);
        const validatedEvent = websocketEventSchema.parse(data);

        switch (validatedEvent.type) {
          case "container.created":
          case "container.started": {
            const eventData = validatedEvent.data as ContainerEventData;
            // Fetch full container details from API
            // For now, just update the state if we have the container
            updateContainer(eventData.container_id, {
              state: eventData.state,
            });
            break;
          }

          case "container.exited": {
            const eventData = validatedEvent.data as ContainerEventData;
            updateContainer(eventData.container_id, {
              state: eventData.state,
              exit_code: eventData.exit_code,
            });
            break;
          }

          case "container.removed": {
            const eventData = validatedEvent.data as ContainerEventData;
            removeContainer(eventData.container_id);
            break;
          }

          case "container.updated": {
            const eventData = validatedEvent.data as ContainerEventData;
            updateContainer(eventData.container_id, {
              state: eventData.state,
            });
            break;
          }

          case "metrics.update": {
            const metricsData = validatedEvent.data as MetricsEventData;
            updateMetrics(metricsData.container_id, {
              container_id: metricsData.container_id,
              timestamp: validatedEvent.timestamp,
              cpu_percent: metricsData.cpu_percent,
              memory_usage_mb: metricsData.memory_usage_mb,
              memory_limit_mb: metricsData.memory_limit_mb,
              memory_percent: metricsData.memory_percent,
              network_rx_bytes: metricsData.network_rx_bytes,
              network_tx_bytes: metricsData.network_tx_bytes,
              block_read_bytes: 0, // Not included in event data
              block_write_bytes: 0, // Not included in event data
            });
            break;
          }

          case "heartbeat":
            // Heartbeat received, connection is alive
            break;

          case "error": {
            console.error("WebSocket error event:", validatedEvent.data);
            break;
          }

          default:
            console.warn("Unknown WebSocket event type:", validatedEvent.type);
        }
      } catch (err) {
        console.error("Failed to parse WebSocket message:", err);
      }
    },
    [updateContainer, removeContainer, updateMetrics]
  );

  const connect = useCallback(() => {
    if (ws.current?.readyState === WebSocket.OPEN) {
      return; // Already connected
    }

    try {
      ws.current = new WebSocket(WS_URL);

      ws.current.onopen = () => {
        console.log("WebSocket connected");
        setConnected(true);
        setError(null);
        reconnectAttemptsRef.current = 0;
      };

      ws.current.onmessage = handleMessage;

      ws.current.onerror = (event) => {
        console.error("WebSocket error:", event);
        setError("WebSocket connection error");
      };

      ws.current.onclose = () => {
        console.log("WebSocket disconnected");
        setConnected(false);

        // Attempt to reconnect
        if (reconnectAttemptsRef.current < MAX_RECONNECT_ATTEMPTS) {
          reconnectAttemptsRef.current += 1;
          setError(
            `Connection lost. Reconnecting... (Attempt ${reconnectAttemptsRef.current}/${MAX_RECONNECT_ATTEMPTS})`
          );

          reconnectTimeoutRef.current = window.setTimeout(() => {
            connect();
          }, RECONNECT_INTERVAL);
        } else {
          setError(
            "Failed to connect after multiple attempts. Please refresh the page."
          );
        }
      };
    } catch (err) {
      console.error("Failed to create WebSocket:", err);
      setError("Failed to establish WebSocket connection");
    }
  }, [handleMessage, setConnected]);

  const disconnect = useCallback(() => {
    if (reconnectTimeoutRef.current) {
      clearTimeout(reconnectTimeoutRef.current);
      reconnectTimeoutRef.current = null;
    }

    if (ws.current) {
      ws.current.close();
      ws.current = null;
    }

    setConnected(false);
  }, [setConnected]);

  useEffect(() => {
    connect();

    return () => {
      disconnect();
    };
  }, [connect, disconnect]);

  return {
    isConnected: useContainerStore((state) => state.isConnected),
    error,
    reconnect: connect,
  };
}
