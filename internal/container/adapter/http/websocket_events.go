package http

import (
	"encoding/json"
	"time"

	"github.com/cyucelen/ignite/internal/container/model"
)

type WebSocketEventType string

const (
	EventTypeContainerCreated WebSocketEventType = "container.created"
	EventTypeContainerStarted WebSocketEventType = "container.started"
	EventTypeContainerExited  WebSocketEventType = "container.exited"
	EventTypeContainerRemoved WebSocketEventType = "container.removed"
	EventTypeContainerUpdated WebSocketEventType = "container.updated"
	EventTypeMetricsUpdate    WebSocketEventType = "metrics.update"
	EventTypeError            WebSocketEventType = "error"
	EventTypeHeartbeat        WebSocketEventType = "heartbeat"
)

type WebSocketEvent struct {
	Type      WebSocketEventType `json:"type"`
	Timestamp time.Time          `json:"timestamp"`
	Data      interface{}        `json:"data"`
}

func NewWebSocketEvent(eventType WebSocketEventType, data interface{}) *WebSocketEvent {
	return &WebSocketEvent{
		Type:      eventType,
		Timestamp: time.Now(),
		Data:      data,
	}
}

func (e *WebSocketEvent) ToJSON() ([]byte, error) {
	return json.Marshal(e)
}

type ContainerEventData struct {
	ContainerID string              `json:"container_id"`
	Image       string              `json:"image"`
	State       model.ContainerState `json:"state"`
	ExitCode    *int                `json:"exit_code,omitempty"`
}

type MetricsEventData struct {
	ContainerID    string  `json:"container_id"`
	CPUPercent     float64 `json:"cpu_percent"`
	MemoryUsageMB  uint64  `json:"memory_usage_mb"`
	MemoryLimitMB  uint64  `json:"memory_limit_mb"`
	MemoryPercent  float64 `json:"memory_percent"`
	NetworkRxBytes uint64  `json:"network_rx_bytes"`
	NetworkTxBytes uint64  `json:"network_tx_bytes"`
}

type ErrorEventData struct {
	Message string `json:"message"`
	Code    string `json:"code,omitempty"`
}

func NewContainerEvent(eventType WebSocketEventType, container *model.Container) *WebSocketEvent {
	data := &ContainerEventData{
		ContainerID: container.ID,
		Image:       container.Image,
		State:       container.State,
	}

	if container.ExitCode != nil {
		data.ExitCode = container.ExitCode
	}

	return NewWebSocketEvent(eventType, data)
}

func NewMetricsEvent(metrics *model.ContainerMetrics) *WebSocketEvent {
	data := &MetricsEventData{
		ContainerID:    metrics.ContainerID,
		CPUPercent:     metrics.CPUPercent,
		MemoryUsageMB:  metrics.MemoryUsageMB,
		MemoryLimitMB:  metrics.MemoryLimitMB,
		MemoryPercent:  metrics.MemoryPercent,
		NetworkRxBytes: metrics.NetworkRxBytes,
		NetworkTxBytes: metrics.NetworkTxBytes,
	}

	return NewWebSocketEvent(EventTypeMetricsUpdate, data)
}

func NewErrorEvent(message string, code string) *WebSocketEvent {
	data := &ErrorEventData{
		Message: message,
		Code:    code,
	}

	return NewWebSocketEvent(EventTypeError, data)
}

func NewHeartbeatEvent() *WebSocketEvent {
	return NewWebSocketEvent(EventTypeHeartbeat, map[string]string{
		"status": "alive",
	})
}
