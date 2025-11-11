package config

import (
	"os"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestLoadConfig_Defaults(t *testing.T) {
	// Clear environment
	os.Clearenv()

	config := LoadConfig()

	assert.Equal(t, "3000", config.Port)
	assert.Equal(t, 50, config.MaxContainers)
	assert.Equal(t, "unix:///var/run/docker.sock", config.DockerHost)
	assert.Equal(t, 300, config.ContainerTimeout)
	assert.Equal(t, 30, config.ShutdownTimeout)
	assert.Equal(t, 5*time.Minute, config.ImagePullTimeout)
}

func TestLoadConfig_FromEnvironment(t *testing.T) {
	// Set environment variables
	require.NoError(t, os.Setenv("PORT", "8080"))
	require.NoError(t, os.Setenv("MAX_CONTAINERS", "100"))
	require.NoError(t, os.Setenv("DOCKER_HOST", "tcp://docker-host:2375"))
	require.NoError(t, os.Setenv("CONTAINER_TIMEOUT", "600"))
	require.NoError(t, os.Setenv("SHUTDOWN_TIMEOUT", "60"))
	require.NoError(t, os.Setenv("IMAGE_PULL_TIMEOUT", "10m"))
	defer os.Clearenv()

	config := LoadConfig()

	assert.Equal(t, "8080", config.Port)
	assert.Equal(t, 100, config.MaxContainers)
	assert.Equal(t, "tcp://docker-host:2375", config.DockerHost)
	assert.Equal(t, 600, config.ContainerTimeout)
	assert.Equal(t, 60, config.ShutdownTimeout)
	assert.Equal(t, 10*time.Minute, config.ImagePullTimeout)
}

func TestLoadConfig_InvalidValues(t *testing.T) {
	// Set invalid values
	require.NoError(t, os.Setenv("MAX_CONTAINERS", "invalid"))
	require.NoError(t, os.Setenv("CONTAINER_TIMEOUT", "not-a-number"))
	require.NoError(t, os.Setenv("SHUTDOWN_TIMEOUT", "abc"))
	require.NoError(t, os.Setenv("IMAGE_PULL_TIMEOUT", "not-a-duration"))
	defer os.Clearenv()

	config := LoadConfig()

	// Should fall back to defaults for invalid values
	assert.Equal(t, 50, config.MaxContainers)
	assert.Equal(t, 300, config.ContainerTimeout)
	assert.Equal(t, 30, config.ShutdownTimeout)
	assert.Equal(t, 5*time.Minute, config.ImagePullTimeout)
}

func TestImagePullTimeout_MaximumEnforcement(t *testing.T) {
	// Set timeout exceeding 15 minutes
	require.NoError(t, os.Setenv("IMAGE_PULL_TIMEOUT", "20m"))
	defer os.Clearenv()

	config := LoadConfig()

	// Should be capped at 15 minutes
	assert.Equal(t, 15*time.Minute, config.ImagePullTimeout)
}

func TestImagePullTimeout_ValidDurations(t *testing.T) {
	tests := []struct {
		name     string
		envValue string
		expected time.Duration
	}{
		{
			name:     "2 minutes",
			envValue: "2m",
			expected: 2 * time.Minute,
		},
		{
			name:     "5 minutes (default)",
			envValue: "5m",
			expected: 5 * time.Minute,
		},
		{
			name:     "10 minutes",
			envValue: "10m",
			expected: 10 * time.Minute,
		},
		{
			name:     "15 minutes (max)",
			envValue: "15m",
			expected: 15 * time.Minute,
		},
		{
			name:     "90 seconds",
			envValue: "90s",
			expected: 90 * time.Second,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			require.NoError(t, os.Setenv("IMAGE_PULL_TIMEOUT", tt.envValue))
			defer os.Clearenv()

			config := LoadConfig()
			assert.Equal(t, tt.expected, config.ImagePullTimeout)
		})
	}
}
