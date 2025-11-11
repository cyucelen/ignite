package model

import (
	"time"
)

type ContainerMetrics struct {
	ContainerID    string    `json:"container_id"`
	Timestamp      time.Time `json:"timestamp"`
	CPUPercent     float64   `json:"cpu_percent"`
	MemoryUsageMB  uint64    `json:"memory_usage_mb"`
	MemoryLimitMB  uint64    `json:"memory_limit_mb"`
	MemoryPercent  float64   `json:"memory_percent"`
	NetworkRxBytes uint64    `json:"network_rx_bytes"`
	NetworkTxBytes uint64    `json:"network_tx_bytes"`
	BlockReadBytes  uint64   `json:"block_read_bytes"`
	BlockWriteBytes uint64   `json:"block_write_bytes"`
}

func NewContainerMetrics(containerID string) *ContainerMetrics {
	return &ContainerMetrics{
		ContainerID: containerID,
		Timestamp:   time.Now(),
	}
}

func (m *ContainerMetrics) IsHealthy() bool {
	if m.CPUPercent > 95.0 {
		return false
	}
	if m.MemoryLimitMB > 0 && m.MemoryPercent > 95.0 {
		return false
	}
	return true
}

func (m *ContainerMetrics) Validate() error {
	if m.ContainerID == "" {
		return ErrInvalidContainerID
	}
	if m.CPUPercent < 0 {
		return ErrInvalidMetrics
	}
	if m.MemoryPercent < 0 || m.MemoryPercent > 100 {
		return ErrInvalidMetrics
	}
	return nil
}
