package http

import (
	"github.com/cyucelen/ignite/internal/container/model"
)

type ContainerResponse struct {
	ID          string            `json:"id"`
	DockerID    string            `json:"docker_id,omitempty"`
	Image       string            `json:"image"`
	Command     []string          `json:"command,omitempty"`
	Environment map[string]string `json:"environment,omitempty"`
	State       string            `json:"state"`
	CreatedAt   string            `json:"created_at"`
	StartedAt   *string           `json:"started_at,omitempty"`
	ExitedAt    *string           `json:"exited_at,omitempty"`
	ExitCode    *int              `json:"exit_code,omitempty"`
	Uptime      *int64            `json:"uptime,omitempty"`
}

type ErrorResponse struct {
	Error string `json:"error"`
}

type HealthCheckResponse struct {
	Status    string `json:"status"`
	Docker    string `json:"docker"`
	Timestamp string `json:"timestamp"`
}

type ListContainersResponse struct {
	Containers []*ContainerResponse `json:"containers"`
	Count      int                  `json:"count"`
}

func NewContainerResponse(container *model.Container) *ContainerResponse {
	response := &ContainerResponse{
		ID:          container.ID,
		DockerID:    container.DockerID,
		Image:       container.Image,
		Command:     container.Command,
		Environment: container.EnvironmentVariables,
		State:       container.State.String(),
		CreatedAt:   container.CreatedAt.Format("2006-01-02T15:04:05Z07:00"),
	}

	if container.StartedAt != nil {
		startedAt := container.StartedAt.Format("2006-01-02T15:04:05Z07:00")
		response.StartedAt = &startedAt
	}

	if container.ExitedAt != nil {
		exitedAt := container.ExitedAt.Format("2006-01-02T15:04:05Z07:00")
		response.ExitedAt = &exitedAt
	}

	if container.ExitCode != nil {
		response.ExitCode = container.ExitCode
	}

	response.Uptime = container.Uptime()

	return response
}

func NewErrorResponse(err error) *ErrorResponse {
	return &ErrorResponse{
		Error: err.Error(),
	}
}
