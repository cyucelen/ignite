import { z } from "zod";

export const containerStateSchema = z.enum([
  "creating",
  "running",
  "exited",
  "stopped",
  "removed",
]);

export const containerSchema = z.object({
  id: z.string().uuid(),
  docker_id: z.string().min(1),
  image: z.string().min(1),
  command: z.array(z.string()).optional(),
  environment: z.record(z.string(), z.string()).optional(),
  state: containerStateSchema,
  created_at: z.string(),
  started_at: z.string().optional(),
  exited_at: z.string().optional(),
  exit_code: z.number().int().optional(),
  last_logs: z.array(z.string()).optional(),
});

export const containerMetricsSchema = z.object({
  container_id: z.string().uuid(),
  timestamp: z.string(),
  cpu_percent: z.number().min(0),
  memory_usage_mb: z.number().int().min(0),
  memory_limit_mb: z.number().int().min(0),
  memory_percent: z.number().min(0).max(100),
  network_rx_bytes: z.number().int().min(0),
  network_tx_bytes: z.number().int().min(0),
  block_read_bytes: z.number().int().min(0),
  block_write_bytes: z.number().int().min(0),
});

export const uiContainerViewSchema = z.object({
  container: containerSchema,
  metrics: containerMetricsSchema.optional(),
  is_healthy: z.boolean(),
});

export const logEntrySchema = z.object({
  container_id: z.string().uuid(),
  stream: z.enum(["stdout", "stderr"]),
  timestamp: z.string(),
  message: z.string(),
  line_number: z.number().int().positive(),
});

export const websocketEventTypeSchema = z.enum([
  "container.created",
  "container.started",
  "container.exited",
  "container.removed",
  "container.updated",
  "metrics.update",
  "error",
  "heartbeat",
]);

export const websocketEventSchema = z.object({
  type: websocketEventTypeSchema,
  timestamp: z.string(),
  data: z.unknown(),
});

export const containerEventDataSchema = z.object({
  container_id: z.string().uuid(),
  image: z.string().min(1),
  state: containerStateSchema,
  exit_code: z.number().int().optional(),
});

export const metricsEventDataSchema = z.object({
  container_id: z.string().uuid(),
  cpu_percent: z.number().min(0),
  memory_usage_mb: z.number().int().min(0),
  memory_limit_mb: z.number().int().min(0),
  memory_percent: z.number().min(0).max(100),
  network_rx_bytes: z.number().int().min(0),
  network_tx_bytes: z.number().int().min(0),
});

export const errorEventDataSchema = z.object({
  message: z.string().min(1),
  code: z.string().optional(),
});

export const createContainerRequestSchema = z.object({
  image: z
    .string()
    .min(1, "Image name is required")
    .regex(
      /^[\w][\w.-]*(?::[\w][\w.-]*)?(?:@[A-Za-z][A-Za-z0-9]*(?:[-_+.][A-Za-z][A-Za-z0-9]*)*:[0-9a-fA-F]{32,})?$/,
      "Invalid image format"
    ),
  command: z.array(z.string()).optional(),
  environment: z
    .record(z.string(), z.string())
    .refine((env) => Object.keys(env).length <= 100, {
      message: "Maximum 100 environment variables allowed",
    })
    .optional(),
});

export const createContainerResponseSchema = z.object({
  id: z.string().uuid(),
  docker_id: z.string().min(1),
  state: containerStateSchema,
  message: z.string(),
});

export const listContainersResponseSchema = z.object({
  containers: z.array(containerSchema),
  count: z.number().int().min(0),
});

export const containerDetailsResponseSchema = z.object({
  container: containerSchema,
  metrics: containerMetricsSchema.optional(),
});

export const deleteContainerResponseSchema = z.object({
  message: z.string().min(1),
});

export const healthCheckResponseSchema = z.object({
  status: z.string(),
  timestamp: z.string(),
  uptime_seconds: z.number().int().min(0),
  containers_count: z.number().int().min(0),
});

export const apiErrorSchema = z.object({
  error: z.string().min(1),
  code: z.string().optional(),
  details: z.string().optional(),
});
