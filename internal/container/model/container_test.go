package model

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestContainerState_String(t *testing.T) {
	tests := []struct {
		state    ContainerState
		expected string
	}{
		{Creating, "creating"},
		{Running, "running"},
		{Exited, "exited"},
		{Stopped, "stopped"},
		{Removed, "removed"},
	}

	for _, tt := range tests {
		t.Run(tt.expected, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.state.String())
		})
	}
}

func TestContainerState_ValidTransitions(t *testing.T) {
	tests := []struct {
		name     string
		from     ContainerState
		to       ContainerState
		expected bool
	}{
		{"Creating to Running", Creating, Running, true},
		{"Creating to Exited", Creating, Exited, true},
		{"Running to Exited", Running, Exited, true},
		{"Running to Stopped", Running, Stopped, true},
		{"Exited to Removed", Exited, Removed, true},
		{"Stopped to Removed", Stopped, Removed, true},

		// Invalid transitions
		{"Exited to Running", Exited, Running, false},
		{"Removed to Running", Removed, Running, false},
		{"Stopped to Running", Stopped, Running, false},
		{"Creating to Removed", Creating, Removed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.from.CanTransitionTo(tt.to))
		})
	}
}

func TestContainerState_IsTerminal(t *testing.T) {
	tests := []struct {
		state    ContainerState
		terminal bool
	}{
		{Creating, false},
		{Running, false},
		{Exited, true},
		{Stopped, false},
		{Removed, true},
	}

	for _, tt := range tests {
		t.Run(tt.state.String(), func(t *testing.T) {
			assert.Equal(t, tt.terminal, tt.state.IsTerminal())
		})
	}
}

func TestContainer_NewContainer(t *testing.T) {
	config := &ContainerConfig{
		Image:       "alpine:latest",
		Command:     []string{"echo", "hello"},
		Environment: map[string]string{"KEY": "value"},
		ResourceLimits: &ResourceLimits{
			MemoryMB: 256,
			CPUCores: 0.5,
		},
	}

	container := NewContainer(config)

	assert.NotEmpty(t, container.ID)
	assert.Equal(t, "alpine:latest", container.Image)
	assert.Equal(t, []string{"echo", "hello"}, container.Command)
	assert.Equal(t, map[string]string{"KEY": "value"}, container.EnvironmentVariables)
	assert.Equal(t, Creating, container.State)
	assert.NotZero(t, container.CreatedAt)
	assert.Nil(t, container.StartedAt)
	assert.Nil(t, container.ExitedAt)
	assert.Nil(t, container.ExitCode)
}

func TestContainer_Start(t *testing.T) {
	container := NewContainer(&ContainerConfig{Image: "alpine:latest"})
	dockerID := "docker-abc123"

	err := container.Start(dockerID)

	assert.NoError(t, err)
	assert.Equal(t, Running, container.State)
	assert.Equal(t, dockerID, container.DockerID)
	assert.NotNil(t, container.StartedAt)
}

func TestContainer_Start_InvalidState(t *testing.T) {
	container := NewContainer(&ContainerConfig{Image: "alpine:latest"})
	container.State = Exited

	err := container.Start("docker-abc123")

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestContainer_MarkExited(t *testing.T) {
	container := NewContainer(&ContainerConfig{Image: "alpine:latest"})
	err := container.Start("docker-abc123")
	assert.NoError(t, err)

	exitCode := 0
	exitTime := time.Now()

	err = container.MarkExited(exitCode, exitTime)

	assert.NoError(t, err)
	assert.Equal(t, Exited, container.State)
	assert.Equal(t, &exitCode, container.ExitCode)
	assert.Equal(t, &exitTime, container.ExitedAt)
}

func TestContainer_MarkExited_InvalidState(t *testing.T) {
	container := NewContainer(&ContainerConfig{Image: "alpine:latest"})
	container.State = Removed

	err := container.MarkExited(0, time.Now())

	assert.Error(t, err)
	assert.ErrorIs(t, err, ErrInvalidState)
}

func TestContainer_Stop(t *testing.T) {
	container := NewContainer(&ContainerConfig{Image: "alpine:latest"})
	err := container.Start("docker-abc123")
	assert.NoError(t, err)

	err = container.Stop()

	assert.NoError(t, err)
	assert.Equal(t, Stopped, container.State)
}

func TestContainer_Remove(t *testing.T) {
	container := NewContainer(&ContainerConfig{Image: "alpine:latest"})
	err := container.Start("docker-abc123")
	assert.NoError(t, err)
	err = container.MarkExited(0, time.Now())
	assert.NoError(t, err)

	err = container.Remove()

	assert.NoError(t, err)
	assert.Equal(t, Removed, container.State)
}

func TestContainer_IsRunning(t *testing.T) {
	tests := []struct {
		name     string
		state    ContainerState
		expected bool
	}{
		{"Creating", Creating, false},
		{"Running", Running, true},
		{"Exited", Exited, false},
		{"Stopped", Stopped, false},
		{"Removed", Removed, false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			container := NewContainer(&ContainerConfig{Image: "alpine:latest"})
			container.State = tt.state

			assert.Equal(t, tt.expected, container.IsRunning())
		})
	}
}

func TestContainer_Uptime(t *testing.T) {
	container := NewContainer(&ContainerConfig{Image: "alpine:latest"})
	startTime := time.Now().Add(-60 * time.Second)
	container.StartedAt = &startTime
	container.State = Running

	uptime := container.Uptime()

	assert.NotNil(t, uptime)
	assert.GreaterOrEqual(t, *uptime, int64(59)) // Allow 1 second tolerance
	assert.LessOrEqual(t, *uptime, int64(61))
}

func TestContainer_Uptime_NotRunning(t *testing.T) {
	container := NewContainer(&ContainerConfig{Image: "alpine:latest"})

	uptime := container.Uptime()

	assert.Nil(t, uptime)
}
// T005: Test for RegistryCredentials struct
func TestRegistryCredentials(t *testing.T) {
	tests := []struct {
		name     string
		creds    *RegistryCredentials
		wantNil  bool
	}{
		{
			name: "valid credentials",
			creds: &RegistryCredentials{
				Username: "testuser",
				Password: "ghp_testtoken123",
			},
			wantNil: false,
		},
		{
			name: "empty username",
			creds: &RegistryCredentials{
				Username: "",
				Password: "ghp_testtoken123",
			},
			wantNil: false,
		},
		{
			name: "empty password",
			creds: &RegistryCredentials{
				Username: "testuser",
				Password: "",
			},
			wantNil: false,
		},
		{
			name:    "nil credentials",
			creds:   nil,
			wantNil: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if tt.wantNil {
				assert.Nil(t, tt.creds)
			} else {
				assert.NotNil(t, tt.creds)
				// Verify fields are accessible
				_ = tt.creds.Username
				_ = tt.creds.Password
			}
		})
	}
}

// T006: Test for ContainerConfig with RegistryAuth field
func TestContainerConfig_WithRegistryAuth(t *testing.T) {
	tests := []struct {
		name   string
		config *ContainerConfig
	}{
		{
			name: "config with registry auth",
			config: &ContainerConfig{
				Image:   "ghcr.io/owner/private-image:latest",
				Command: []string{"echo", "hello"},
				RegistryAuth: &RegistryCredentials{
					Username: "testuser",
					Password: "ghp_testtoken123",
				},
			},
		},
		{
			name: "config without registry auth (nil)",
			config: &ContainerConfig{
				Image:        "nginx:latest",
				Command:      []string{"nginx"},
				RegistryAuth: nil,
			},
		},
		{
			name: "config with all fields",
			config: &ContainerConfig{
				Image:   "ghcr.io/owner/image:tag",
				Command: []string{"./app"},
				Environment: map[string]string{
					"ENV": "test",
				},
				ResourceLimits: &ResourceLimits{
					MemoryMB: 512,
					CPUCores: 1.0,
				},
				RegistryAuth: &RegistryCredentials{
					Username: "user",
					Password: "pass",
				},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.NotNil(t, tt.config)
			assert.NotEmpty(t, tt.config.Image)

			// Verify RegistryAuth field is accessible
			if tt.config.RegistryAuth != nil {
				assert.NotNil(t, tt.config.RegistryAuth)
				assert.NotEmpty(t, tt.config.RegistryAuth.Username)
			}

			// Verify config can be used to create container
			container := NewContainer(tt.config)
			assert.NotNil(t, container)
			assert.Equal(t, tt.config.Image, container.Image)
			assert.Equal(t, tt.config.Command, container.Command)
		})
	}
}
