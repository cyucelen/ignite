package service

import (
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T024: Unit tests for MapDockerState() - all valid states
func TestMapDockerState_ValidStates(t *testing.T) {
	tests := []struct {
		name         string
		dockerState  *types.ContainerState
		expectedState model.ContainerState
		shouldMap    bool
	}{
		{
			name:          "created state maps to Creating",
			dockerState:   &types.ContainerState{Status: "created"},
			expectedState: model.Creating,
			shouldMap:     true,
		},
		{
			name:          "running state maps to Running",
			dockerState:   &types.ContainerState{Status: "running", Running: true},
			expectedState: model.Running,
			shouldMap:     true,
		},
		{
			name:          "paused state maps to Running",
			dockerState:   &types.ContainerState{Status: "paused", Paused: true},
			expectedState: model.Running,
			shouldMap:     true,
		},
		{
			name:          "exited state maps to Exited",
			dockerState:   &types.ContainerState{Status: "exited", ExitCode: 0},
			expectedState: model.Exited,
			shouldMap:     true,
		},
		{
			name:          "dead state maps to Exited",
			dockerState:   &types.ContainerState{Status: "dead", Dead: true},
			expectedState: model.Exited,
			shouldMap:     true,
		},
		{
			name:          "restarting state maps to Creating",
			dockerState:   &types.ContainerState{Status: "restarting", Restarting: true},
			expectedState: model.Creating,
			shouldMap:     true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			state, ok := MapDockerState(tt.dockerState)
			assert.Equal(t, tt.shouldMap, ok)
			if ok {
				assert.Equal(t, tt.expectedState, state)
			}
		})
	}
}

// T025: Unit tests for MapDockerState() - removing state
func TestMapDockerState_RemovingState(t *testing.T) {
	dockerState := &types.ContainerState{Status: "removing"}
	state, ok := MapDockerState(dockerState)
	assert.False(t, ok, "removing state should not be mappable")
	assert.Equal(t, model.ContainerState(0), state)
}

// T026: Unit tests for MapDockerState() - unknown state
func TestMapDockerState_UnknownState(t *testing.T) {
	dockerState := &types.ContainerState{Status: "unknown-future-state"}
	state, ok := MapDockerState(dockerState)
	assert.False(t, ok, "unknown state should not be mappable")
	assert.Equal(t, model.ContainerState(0), state)
}

// T027: Unit tests for parseOptionalTime() - various inputs
func TestParseOptionalTime(t *testing.T) {
	tests := []struct {
		name     string
		input    string
		expected *time.Time
	}{
		{
			name:     "valid RFC3339 timestamp",
			input:    "2025-11-10T15:30:45Z",
			expected: timePtr(time.Date(2025, 11, 10, 15, 30, 45, 0, time.UTC)),
		},
		{
			name:     "empty string returns nil",
			input:    "",
			expected: nil,
		},
		{
			name:     "zero time returns nil",
			input:    "0001-01-01T00:00:00Z",
			expected: nil,
		},
		{
			name:     "invalid format returns nil",
			input:    "not-a-timestamp",
			expected: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseOptionalTime(tt.input)
			if tt.expected == nil {
				assert.Nil(t, result)
			} else {
				require.NotNil(t, result)
				assert.Equal(t, *tt.expected, *result)
			}
		})
	}
}

// T028: Unit tests for parseDockerEnv() - various inputs
func TestParseDockerEnv(t *testing.T) {
	tests := []struct {
		name     string
		input    []string
		expected map[string]string
	}{
		{
			name:  "valid env array",
			input: []string{"KEY1=value1", "KEY2=value2", "KEY3=value3"},
			expected: map[string]string{
				"KEY1": "value1",
				"KEY2": "value2",
				"KEY3": "value3",
			},
		},
		{
			name:     "empty array",
			input:    []string{},
			expected: map[string]string{},
		},
		{
			name:     "nil array",
			input:    nil,
			expected: map[string]string{},
		},
		{
			name:  "value with equals sign",
			input: []string{"PATH=/usr/bin:/bin"},
			expected: map[string]string{
				"PATH": "/usr/bin:/bin",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := parseDockerEnv(tt.input)
			assert.Equal(t, tt.expected, result)
		})
	}
}

// T029: Unit tests for ReconstructContainer() - complete running container
func TestReconstructContainer_RunningContainer(t *testing.T) {
	dockerJSON := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      "docker123",
			Created: "2025-11-10T10:00:00Z",
			State: &types.ContainerState{
				Status:     "running",
				Running:    true,
				StartedAt:  "2025-11-10T10:00:05Z",
				FinishedAt: "",
			},
		},
		Config: &container.Config{
			Image: "alpine:latest",
			Cmd:   []string{"sleep", "3600"},
			Env:   []string{"TEST=value"},
			Labels: map[string]string{
				"ignite.managed":    "true",
				"ignite.id":         "f47ac10b-58cc-4372-a567-0e02b2c3d479",
				"ignite.created-at": "2025-11-10T10:00:00Z",
				"ignite.image":      "alpine:latest",
				"ignite.version":    "1.0.0",
			},
		},
	}

	container, err := ReconstructContainer(dockerJSON)

	require.NoError(t, err)
	assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", container.ID)
	assert.Equal(t, "docker123", container.DockerID)
	assert.Equal(t, "alpine:latest", container.Image)
	assert.Equal(t, []string{"sleep", "3600"}, container.Command)
	assert.Equal(t, model.Running, container.State)
	assert.NotNil(t, container.StartedAt)
	assert.Nil(t, container.ExitedAt)
	assert.Nil(t, container.ExitCode)
	assert.NotNil(t, container.ReconciledAt, "ReconciledAt should be set")
	assert.Equal(t, map[string]string{"TEST": "value"}, container.EnvironmentVariables)
}

// T030: Unit tests for ReconstructContainer() - exited container with exit code
func TestReconstructContainer_ExitedContainer(t *testing.T) {
	dockerJSON := types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			ID:      "docker456",
			Created: "2025-11-10T10:00:00Z",
			State: &types.ContainerState{
				Status:     "exited",
				Running:    false,
				StartedAt:  "2025-11-10T10:00:05Z",
				FinishedAt: "2025-11-10T10:05:00Z",
				ExitCode:   137,
			},
		},
		Config: &container.Config{
			Image: "alpine:latest",
			Cmd:   []string{"sh", "-c", "exit 137"},
			Env:   []string{},
			Labels: map[string]string{
				"ignite.managed":    "true",
				"ignite.id":         "a1b2c3d4-e5f6-4789-a012-345678901234",
				"ignite.created-at": "2025-11-10T10:00:00Z",
				"ignite.image":      "alpine:latest",
				"ignite.version":    "1.0.0",
			},
		},
	}

	container, err := ReconstructContainer(dockerJSON)

	require.NoError(t, err)
	assert.Equal(t, "a1b2c3d4-e5f6-4789-a012-345678901234", container.ID)
	assert.Equal(t, model.Exited, container.State)
	assert.NotNil(t, container.ExitedAt)
	require.NotNil(t, container.ExitCode)
	assert.Equal(t, 137, *container.ExitCode)
	assert.NotNil(t, container.ReconciledAt)
}

// T031: Unit tests for ReconstructContainer() - invalid labels
func TestReconstructContainer_InvalidLabels(t *testing.T) {
	tests := []struct {
		name   string
		labels map[string]string
	}{
		{
			name: "missing ignite.managed",
			labels: map[string]string{
				"ignite.id":         "f47ac10b-58cc-4372-a567-0e02b2c3d479",
				"ignite.created-at": "2025-11-10T10:00:00Z",
				"ignite.image":      "alpine:latest",
				"ignite.version":    "1.0.0",
			},
		},
		{
			name: "invalid UUID",
			labels: map[string]string{
				"ignite.managed":    "true",
				"ignite.id":         "not-a-uuid",
				"ignite.created-at": "2025-11-10T10:00:00Z",
				"ignite.image":      "alpine:latest",
				"ignite.version":    "1.0.0",
			},
		},
		{
			name: "invalid timestamp",
			labels: map[string]string{
				"ignite.managed":    "true",
				"ignite.id":         "f47ac10b-58cc-4372-a567-0e02b2c3d479",
				"ignite.created-at": "invalid-date",
				"ignite.image":      "alpine:latest",
				"ignite.version":    "1.0.0",
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			dockerJSON := types.ContainerJSON{
				ContainerJSONBase: &types.ContainerJSONBase{
					ID:      "docker789",
					Created: "2025-11-10T10:00:00Z",
					State: &types.ContainerState{
						Status:  "running",
						Running: true,
					},
				},
				Config: &container.Config{
					Image:  "alpine:latest",
					Cmd:    []string{"sleep", "3600"},
					Labels: tt.labels,
				},
			}

			_, err := ReconstructContainer(dockerJSON)
			assert.Error(t, err, "should return error for invalid labels")
		})
	}
}

// Helper function to create time pointer
func timePtr(t time.Time) *time.Time {
	return &t
}
