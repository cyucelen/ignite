import type {
  ContainerMetrics,
  CreateContainerRequest,
  CreateContainerResponse,
  ListContainersResponse,
  ContainerDetailsResponse,
  DeleteContainerResponse,
  HealthCheckResponse,
} from "./types";

import {
  createContainerRequestSchema,
  createContainerResponseSchema,
  listContainersResponseSchema,
  containerDetailsResponseSchema,
  deleteContainerResponseSchema,
  healthCheckResponseSchema,
  apiErrorSchema,
} from "./schemas";

// API client configuration
const API_BASE_URL = "/api";

// Custom error class for API errors
export class APIClientError extends Error {
  public statusCode: number;
  public code?: string;
  public details?: string;

  constructor(
    message: string,
    statusCode: number,
    code?: string,
    details?: string
  ) {
    super(message);
    this.name = "APIClientError";
    this.statusCode = statusCode;
    this.code = code;
    this.details = details;
  }
}

// Generic fetch wrapper with error handling
async function apiRequest<T>(
  endpoint: string,
  options: RequestInit = {},
  schema?: (data: unknown) => T
): Promise<T> {
  const url = `${API_BASE_URL}${endpoint}`;

  try {
    const response = await fetch(url, {
      ...options,
      headers: {
        "Content-Type": "application/json",
        ...options.headers,
      },
    });

    // Handle 204 No Content responses (no body to parse)
    if (response.status === 204) {
      return {} as T;
    }

    const data = await response.json();

    if (!response.ok) {
      // Parse error response
      const errorResult = apiErrorSchema.safeParse(data);
      if (errorResult.success) {
        throw new APIClientError(
          errorResult.data.error,
          response.status,
          errorResult.data.code,
          errorResult.data.details
        );
      }
      throw new APIClientError(
        data.error || "An unknown error occurred",
        response.status
      );
    }

    // Validate response with schema if provided
    if (schema) {
      return schema(data);
    }

    return data as T;
  } catch (error) {
    if (error instanceof APIClientError) {
      throw error;
    }
    if (error instanceof Error) {
      throw new APIClientError(error.message, 0);
    }
    throw new APIClientError("An unexpected error occurred", 0);
  }
}

// Container API methods
export const containerAPI = {
  /**
   * Create a new container
   */
  async create(
    request: CreateContainerRequest
  ): Promise<CreateContainerResponse> {
    // Validate request
    const validatedRequest = createContainerRequestSchema.parse(request);

    return apiRequest<CreateContainerResponse>(
      "/containers",
      {
        method: "POST",
        body: JSON.stringify(validatedRequest),
      },
      (data) => createContainerResponseSchema.parse(data)
    );
  },

  /**
   * List all containers
   */
  async list(): Promise<ListContainersResponse> {
    return apiRequest<ListContainersResponse>(
      "/containers",
      { method: "GET" },
      (data) => listContainersResponseSchema.parse(data)
    );
  },

  /**
   * Get container details by ID
   */
  async get(id: string): Promise<ContainerDetailsResponse> {
    return apiRequest<ContainerDetailsResponse>(
      `/containers/${id}`,
      { method: "GET" },
      (data) => containerDetailsResponseSchema.parse(data)
    );
  },

  /**
   * Stop a container by ID
   */
  async stop(id: string): Promise<{ message: string }> {
    return apiRequest<{ message: string }>(
      `/containers/${id}/stop`,
      { method: "POST" }
    );
  },

  /**
   * Delete a container by ID
   */
  async delete(id: string): Promise<DeleteContainerResponse> {
    return apiRequest<DeleteContainerResponse>(
      `/containers/${id}`,
      { method: "DELETE" },
      (data) => deleteContainerResponseSchema.parse(data)
    );
  },

  /**
   * Get container metrics by ID
   */
  async getMetrics(id: string): Promise<ContainerMetrics> {
    return apiRequest<ContainerMetrics>(
      `/containers/${id}/metrics`,
      { method: "GET" }
    );
  },

  /**
   * Get container logs by ID
   */
  async getLogs(
    id: string,
    options?: {
      tail?: number;
      timestamps?: boolean;
    }
  ): Promise<{ logs: string[] }> {
    const params = new URLSearchParams();
    if (options?.tail) params.append("tail", options.tail.toString());
    if (options?.timestamps !== undefined)
      params.append("timestamps", options.timestamps.toString());

    const query = params.toString();
    const endpoint = `/containers/${id}/logs${query ? `?${query}` : ""}`;

    return apiRequest<{ logs: string[] }>(endpoint, { method: "GET" });
  },
};

// Health check API
export const healthAPI = {
  /**
   * Check API health status
   */
  async check(): Promise<HealthCheckResponse> {
    return apiRequest<HealthCheckResponse>(
      "/health",
      { method: "GET" },
      (data) => healthCheckResponseSchema.parse(data)
    );
  },
};

// Export all API methods
export const api = {
  containers: containerAPI,
  health: healthAPI,
};
