import { useEffect, useRef } from "react";
import type { ReactNode } from "react";
import { useWebSocket } from "../hooks/useWebSocket";

interface WebSocketProviderProps {
  children: ReactNode;
}

export function WebSocketProvider({ children }: WebSocketProviderProps) {
  const { isConnected, error } = useWebSocket();
  const hasShownConnected = useRef(false);
  const lastError = useRef<string | null>(null);

  // Log connection status changes (removed toast to prevent infinite loops)
  useEffect(() => {
    if (error && error !== lastError.current) {
      lastError.current = error;
      console.log("WebSocket Error:", error);
    }
  }, [error]);

  useEffect(() => {
    if (isConnected && !hasShownConnected.current) {
      hasShownConnected.current = true;
      console.log("WebSocket: Connected - Real-time updates enabled");
    } else if (!isConnected && hasShownConnected.current) {
      hasShownConnected.current = false;
      console.log("WebSocket: Disconnected");
    }
  }, [isConnected]);

  return <>{children}</>;
}
