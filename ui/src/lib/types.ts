export type ContainerState =
  | "creating"
  | "running"
  | "exited"
  | "stopped"
  | "removed";

export interface Container {
  id: string;
  docker_id: string;
  image: string;
  command?: string[];
  environment?: Record<string, string>;
  state: ContainerState;
  created_at: string;
  started_at?: string;
  exited_at?: string;
  exit_code?: number;
  last_logs?: string[];
}

export interface ContainerMetrics {
  container_id: string;
  timestamp: string;
  cpu_percent: number;
  memory_usage_mb: number;
  memory_limit_mb: number;
  memory_percent: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
  block_read_bytes: number;
  block_write_bytes: number;
}

export interface UIContainerView {
  container: Container;
  metrics?: ContainerMetrics;
  is_healthy: boolean;
}

export interface LogEntry {
  container_id: string;
  stream: "stdout" | "stderr";
  timestamp: string;
  message: string;
  line_number: number;
}

export type WebSocketEventType =
  | "container.created"
  | "container.started"
  | "container.exited"
  | "container.removed"
  | "container.updated"
  | "metrics.update"
  | "error"
  | "heartbeat";

export interface WebSocketEvent<T = unknown> {
  type: WebSocketEventType;
  timestamp: string;
  data: T;
}

export interface ContainerEventData {
  container_id: string;
  image: string;
  state: ContainerState;
  exit_code?: number;
}

export interface MetricsEventData {
  container_id: string;
  cpu_percent: number;
  memory_usage_mb: number;
  memory_limit_mb: number;
  memory_percent: number;
  network_rx_bytes: number;
  network_tx_bytes: number;
}

export interface ErrorEventData {
  message: string;
  code?: string;
}

export interface CreateContainerRequest {
  image: string;
  command?: string[];
  environment?: Record<string, string>;
}

export interface CreateContainerResponse {
  id: string;
  docker_id: string;
  state: ContainerState;
  message: string;
}

export interface ListContainersResponse {
  containers: Container[];
  count: number;
}

export interface ContainerDetailsResponse {
  container: Container;
  metrics?: ContainerMetrics;
}

export interface DeleteContainerResponse {
  message: string;
}

export interface HealthCheckResponse {
  status: string;
  timestamp: string;
  uptime_seconds: number;
  containers_count: number;
}

export interface APIError {
  error: string;
  code?: string;
  details?: string;
}
