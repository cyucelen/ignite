package service

import (
	"fmt"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/cyucelen/ignite/internal/container/model"
)

func MapDockerState(dockerState *types.ContainerState) (model.ContainerState, bool) {
	switch dockerState.Status {
	case "created":
		return model.Creating, true
	case "running":
		return model.Running, true
	case "paused":
		return model.Running, true
	case "exited":
		return model.Exited, true
	case "dead":
		return model.Exited, true
	case "restarting":
		return model.Creating, true
	case "removing":
		return 0, false
	default:
		return 0, false
	}
}

func parseOptionalTime(rfc3339 string) *time.Time {
	if rfc3339 == "" || rfc3339 == "0001-01-01T00:00:00Z" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, rfc3339)
	if err != nil {
		return nil
	}
	return &t
}

func parseDockerEnv(envArray []string) map[string]string {
	if len(envArray) == 0 {
		return map[string]string{}
	}

	env := make(map[string]string)
	for _, pair := range envArray {
		parts := strings.SplitN(pair, "=", 2)
		if len(parts) == 2 {
			env[parts[0]] = parts[1]
		}
	}
	return env
}

func ReconstructContainer(dockerJSON types.ContainerJSON) (*model.Container, error) {
	labels, err := model.ParseLabels(dockerJSON.Config.Labels)
	if err != nil {
		return nil, fmt.Errorf("failed to parse labels: %w", err)
	}

	state, ok := MapDockerState(dockerJSON.State)
	if !ok {
		return nil, fmt.Errorf("unmappable Docker state: %s", dockerJSON.State.Status)
	}

	createdAt, err := time.Parse(time.RFC3339, dockerJSON.Created)
	if err != nil {
		createdAt = labels.CreatedAt
	}
	startedAt := parseOptionalTime(dockerJSON.State.StartedAt)
	exitedAt := parseOptionalTime(dockerJSON.State.FinishedAt)

	var exitCode *int
	if state == model.Exited {
		code := dockerJSON.State.ExitCode
		exitCode = &code
	}

	env := parseDockerEnv(dockerJSON.Config.Env)

	now := time.Now()
	container := &model.Container{
		ID:                   labels.ID,
		DockerID:             dockerJSON.ID,
		Image:                labels.Image,
		Command:              dockerJSON.Config.Cmd,
		EnvironmentVariables: env,
		State:                state,
		CreatedAt:            createdAt,
		StartedAt:            startedAt,
		ExitedAt:             exitedAt,
		ExitCode:             exitCode,
		ReconciledAt:         &now,
		Labels:               dockerJSON.Config.Labels,
	}

	return container, nil
}
