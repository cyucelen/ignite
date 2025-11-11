//go:build integration

package docker

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/cyucelen/ignite/internal/shared/config"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// integrationTestConfig returns a basic config for integration tests
func integrationTestConfig() *config.Config {
	return &config.Config{
		ImagePullTimeout: 5 * time.Minute,
	}
}

// TestDockerManager_FullLifecycle tests the complete container lifecycle with real Docker
// This test requires Docker to be running and available
func TestDockerManager_FullLifecycle(t *testing.T) {
	// Skip if Docker is not available
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer dockerClient.Close()

	// Ping Docker to ensure it's responsive
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		t.Skip("Docker daemon not responding:", err)
	}

	// Create manager with real Docker client
	manager := NewDockerManager(dockerClient, integrationTestConfig())

	// Create a simple container that will exit immediately
	container := &model.Container{
		ID:    "test-container-123",
		Image: "alpine:latest",
		Command: []string{
			"sh", "-c", "echo 'Hello from integration test' && exit 0",
		},
		EnvironmentVariables: map[string]string{
			"TEST_ENV": "integration",
		},
		State: model.Creating,
		Labels: map[string]string{
			"ignite.managed": "true",
			"ignite.test":    "true",
		},
	}

	// Step 1: Create container
	dockerID, err := manager.CreateContainer(ctx, container, nil)
	require.NoError(t, err, "Failed to create container")
	require.NotEmpty(t, dockerID, "Docker ID should not be empty")
	t.Logf("Created container with Docker ID: %s", dockerID)

	// Ensure cleanup
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = manager.RemoveContainer(cleanupCtx, dockerID)
		t.Logf("Cleaned up container: %s", dockerID)
	}()

	// Step 2: Start container
	err = manager.StartContainer(ctx, dockerID)
	require.NoError(t, err, "Failed to start container")
	t.Logf("Started container: %s", dockerID)

	// Step 3: Wait for container to exit (it should exit quickly)
	// In a real scenario, we would use event monitoring, but for this test
	// we'll poll the container status
	maxWaitTime := 30 * time.Second
	checkInterval := 500 * time.Millisecond
	deadline := time.Now().Add(maxWaitTime)

	var containerExited bool
	for time.Now().Before(deadline) {
		inspect, err := dockerClient.ContainerInspect(ctx, dockerID)
		require.NoError(t, err, "Failed to inspect container")

		if !inspect.State.Running {
			containerExited = true
			t.Logf("Container exited with code: %d", inspect.State.ExitCode)
			assert.Equal(t, 0, inspect.State.ExitCode, "Container should exit with code 0")
			break
		}

		time.Sleep(checkInterval)
	}

	assert.True(t, containerExited, "Container should have exited within %v", maxWaitTime)

	// Step 4: Remove container
	err = manager.RemoveContainer(ctx, dockerID)
	require.NoError(t, err, "Failed to remove container")
	t.Logf("Removed container: %s", dockerID)

	// Step 5: Verify container is gone
	_, err = dockerClient.ContainerInspect(ctx, dockerID)
	assert.Error(t, err, "Container should not exist after removal")
}

// TestDockerManager_CreateStartStop tests creating, starting, and stopping a container
func TestDockerManager_CreateStartStop(t *testing.T) {
	// Skip if Docker is not available
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer dockerClient.Close()

	// Ping Docker
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		t.Skip("Docker daemon not responding:", err)
	}

	manager := NewDockerManager(dockerClient, integrationTestConfig())

	// Create a long-running container
	container := &model.Container{
		ID:    "test-container-456",
		Image: "nginx:alpine",
		Command: []string{
			"sh", "-c", "sleep 3600", // Sleep for 1 hour
		},
		State: model.Creating,
		Labels: map[string]string{
			"ignite.managed": "true",
			"ignite.test":    "true",
		},
	}

	// Create container
	dockerID, err := manager.CreateContainer(ctx, container, nil)
	require.NoError(t, err)
	require.NotEmpty(t, dockerID)
	t.Logf("Created container: %s", dockerID)

	// Ensure cleanup
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
		defer cancel()
		_ = manager.StopContainer(cleanupCtx, dockerID)
		_ = manager.RemoveContainer(cleanupCtx, dockerID)
		t.Logf("Cleaned up container: %s", dockerID)
	}()

	// Start container
	err = manager.StartContainer(ctx, dockerID)
	require.NoError(t, err)
	t.Logf("Started container: %s", dockerID)

	// Verify container is running
	inspect, err := dockerClient.ContainerInspect(ctx, dockerID)
	require.NoError(t, err)
	assert.True(t, inspect.State.Running, "Container should be running")

	// Stop container (nginx needs more time for graceful shutdown)
	stopCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	err = manager.StopContainer(stopCtx, dockerID)
	require.NoError(t, err)
	t.Logf("Stopped container: %s", dockerID)

	// Verify container is stopped
	inspect, err = dockerClient.ContainerInspect(ctx, dockerID)
	require.NoError(t, err)
	assert.False(t, inspect.State.Running, "Container should not be running")
}

// TestDockerManager_EventStream tests monitoring container events
func TestDockerManager_EventStream(t *testing.T) {
	// Skip if Docker is not available
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer dockerClient.Close()

	// Ping Docker
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		t.Skip("Docker daemon not responding:", err)
	}

	manager := NewDockerManager(dockerClient, integrationTestConfig())

	// Start watching events
	eventCtx, eventCancel := context.WithCancel(ctx)
	defer eventCancel()

	eventChan, errChan := manager.WatchEvents(eventCtx)

	// Create and start a container that will exit
	container := &model.Container{
		ID:    "test-container-789",
		Image: "alpine:latest",
		Command: []string{
			"sh", "-c", "echo 'Event test' && exit 0",
		},
		State: model.Creating,
		Labels: map[string]string{
			"ignite.managed": "true",
			"ignite.test":    "true",
		},
	}

	dockerID, err := manager.CreateContainer(ctx, container, nil)
	require.NoError(t, err)
	require.NotEmpty(t, dockerID)
	t.Logf("Created container for event test: %s", dockerID)

	// Ensure cleanup
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = manager.RemoveContainer(cleanupCtx, dockerID)
		t.Logf("Cleaned up event test container: %s", dockerID)
	}()

	// Start container
	err = manager.StartContainer(ctx, dockerID)
	require.NoError(t, err)
	t.Logf("Started container for event test: %s", dockerID)

	// Wait for events with timeout
	eventTimeout := time.After(30 * time.Second)
	var receivedStartEvent bool
	var receivedDieEvent bool

	for !receivedStartEvent || !receivedDieEvent {
		select {
		case event := <-eventChan:
			if event.DockerID == dockerID {
				t.Logf("Received event: %s for container %s", event.EventType, dockerID)
				switch event.EventType {
				case "start":
					receivedStartEvent = true
				case "die":
					receivedDieEvent = true
					assert.NotNil(t, event.ExitCode, "Exit code should be present for die event")
				}
			}
		case err := <-errChan:
			t.Logf("Event stream error: %v", err)
		case <-eventTimeout:
			t.Fatal("Timeout waiting for container events")
		}

		if receivedStartEvent && receivedDieEvent {
			break
		}
	}

	assert.True(t, receivedStartEvent, "Should receive start event")
	assert.True(t, receivedDieEvent, "Should receive die event")
}

// TestDockerManager_ResourceLimits tests container creation with resource limits
func TestDockerManager_ResourceLimits(t *testing.T) {
	// Skip if Docker is not available
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer dockerClient.Close()

	// Ping Docker
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		t.Skip("Docker daemon not responding:", err)
	}

	manager := NewDockerManager(dockerClient, integrationTestConfig())

	// Create container with resource limits
	container := &model.Container{
		ID:    "test-container-limits",
		Image: "alpine:latest",
		Command: []string{
			"sh", "-c", "echo 'Resource limits test' && exit 0",
		},
		State: model.Creating,
		ResourceLimits: &model.ResourceLimits{
			MemoryMB: 128, // 128 MB
			CPUCores: 0.5, // 0.5 CPU cores
		},
		Labels: map[string]string{
			"ignite.managed": "true",
			"ignite.test":    "true",
		},
	}

	dockerID, err := manager.CreateContainer(ctx, container, nil)
	require.NoError(t, err)
	require.NotEmpty(t, dockerID)
	t.Logf("Created container with resource limits: %s", dockerID)

	// Ensure cleanup
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = manager.RemoveContainer(cleanupCtx, dockerID)
		t.Logf("Cleaned up resource limits test container: %s", dockerID)
	}()

	// Inspect container to verify resource limits
	inspect, err := dockerClient.ContainerInspect(ctx, dockerID)
	require.NoError(t, err)

	// Verify memory limit (128 MB = 134217728 bytes)
	expectedMemory := int64(128 * 1024 * 1024)
	assert.Equal(t, expectedMemory, inspect.HostConfig.Memory,
		"Memory limit should be set to %d bytes", expectedMemory)

	// Verify CPU quota (0.5 cores = 50000 quota with 100000 period)
	// CPUQuota = CPUCores * CPUPeriod
	if inspect.HostConfig.CPUPeriod > 0 {
		expectedQuota := int64(0.5 * float64(inspect.HostConfig.CPUPeriod))
		assert.Equal(t, expectedQuota, inspect.HostConfig.CPUQuota,
			"CPU quota should be set correctly for 0.5 cores")
	}

	// Start and wait for container to exit
	err = manager.StartContainer(ctx, dockerID)
	require.NoError(t, err)

	// Wait for exit
	maxWaitTime := 10 * time.Second
	checkInterval := 200 * time.Millisecond
	deadline := time.Now().Add(maxWaitTime)

	for time.Now().Before(deadline) {
		inspect, err = dockerClient.ContainerInspect(ctx, dockerID)
		require.NoError(t, err)

		if !inspect.State.Running {
			break
		}

		time.Sleep(checkInterval)
	}

	// Remove container
	err = manager.RemoveContainer(ctx, dockerID)
	require.NoError(t, err)
}

// TestDockerManager_MultipleContainers tests managing multiple containers simultaneously
func TestDockerManager_MultipleContainers(t *testing.T) {
	// Skip if Docker is not available
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer dockerClient.Close()

	// Ping Docker
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		t.Skip("Docker daemon not responding:", err)
	}

	manager := NewDockerManager(dockerClient, integrationTestConfig())

	numContainers := 3
	dockerIDs := make([]string, numContainers)

	// Create multiple containers
	for i := 0; i < numContainers; i++ {
		container := &model.Container{
			ID:    "test-multi-container-" + string(rune('a'+i)),
			Image: "alpine:latest",
			Command: []string{
				"sh", "-c", "echo 'Container test' && sleep 1 && exit 0",
			},
			State: model.Creating,
			Labels: map[string]string{
				"ignite.managed": "true",
				"ignite.test":    "true",
			},
		}

		dockerID, err := manager.CreateContainer(ctx, container, nil)
		require.NoError(t, err)
		require.NotEmpty(t, dockerID)
		dockerIDs[i] = dockerID
		t.Logf("Created container %d: %s", i+1, dockerID)
	}

	// Ensure cleanup
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
		defer cancel()
		for i, dockerID := range dockerIDs {
			_ = manager.StopContainer(cleanupCtx, dockerID)
			_ = manager.RemoveContainer(cleanupCtx, dockerID)
			t.Logf("Cleaned up container %d: %s", i+1, dockerID)
		}
	}()

	// Start all containers
	for i, dockerID := range dockerIDs {
		err := manager.StartContainer(ctx, dockerID)
		require.NoError(t, err)
		t.Logf("Started container %d: %s", i+1, dockerID)
	}

	// Verify all containers are running or have exited
	time.Sleep(2 * time.Second)

	for i, dockerID := range dockerIDs {
		inspect, err := dockerClient.ContainerInspect(ctx, dockerID)
		require.NoError(t, err)
		t.Logf("Container %d (%s) running: %v", i+1, dockerID, inspect.State.Running)
	}

	// Clean up all containers
	for i, dockerID := range dockerIDs {
		err := manager.RemoveContainer(ctx, dockerID)
		require.NoError(t, err)
		t.Logf("Removed container %d: %s", i+1, dockerID)
	}
}

// TestDockerManager_IgniteLabels tests that Ignite labels are correctly attached to containers (T021)
func TestDockerManager_IgniteLabels(t *testing.T) {
	// Skip if Docker is not available
	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		t.Skip("Docker not available:", err)
	}
	defer dockerClient.Close()

	// Ping Docker
	_, err = dockerClient.Ping(ctx)
	if err != nil {
		t.Skip("Docker daemon not responding:", err)
	}

	manager := NewDockerManager(dockerClient, integrationTestConfig())

	// Create a container using NewContainer (which calls CreateLabels internally)
	config := &model.ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"sh", "-c", "echo 'Label test' && exit 0"},
	}
	container := model.NewContainer(config)

	// Create container in Docker
	dockerID, err := manager.CreateContainer(ctx, container, nil)
	require.NoError(t, err)
	require.NotEmpty(t, dockerID)
	t.Logf("Created container with Docker ID: %s", dockerID)

	// Ensure cleanup
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = manager.RemoveContainer(cleanupCtx, dockerID)
		t.Logf("Cleaned up container: %s", dockerID)
	}()

	// Inspect container to verify labels
	inspect, err := dockerClient.ContainerInspect(ctx, dockerID)
	require.NoError(t, err)

	// Verify all 5 Ignite labels are present
	labels := inspect.Config.Labels
	t.Logf("Container labels: %+v", labels)

	// Check ignite.managed
	assert.Equal(t, "true", labels[model.LabelManaged],
		"ignite.managed label should be 'true'")

	// Check ignite.id (should match container ID)
	assert.Equal(t, container.ID, labels[model.LabelID],
		"ignite.id label should match container ID")

	// Check ignite.created-at (should be valid RFC3339 timestamp)
	createdAtStr, ok := labels[model.LabelCreatedAt]
	assert.True(t, ok, "ignite.created-at label should exist")
	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	assert.NoError(t, err, "ignite.created-at should be valid RFC3339 timestamp")
	assert.WithinDuration(t, time.Now(), createdAt, 5*time.Second,
		"ignite.created-at should be recent")

	// Check ignite.image
	assert.Equal(t, "alpine:latest", labels[model.LabelImage],
		"ignite.image label should match image name")

	// Check ignite.version
	assert.NotEmpty(t, labels[model.LabelVersion],
		"ignite.version label should be set")
	assert.Equal(t, model.IgniteVersion, labels[model.LabelVersion],
		"ignite.version label should match IgniteVersion constant")

	t.Logf("All Ignite labels verified successfully")
}

// T011: Integration test for private GHCR image pull with valid credentials
func TestDockerManager_PrivateGHCRImagePull_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	// This test requires GHCR credentials to be provided via environment variables
	// GHCR_USERNAME: GitHub username
	// GHCR_TOKEN: GitHub Personal Access Token with read:packages scope
	ghcrUsername := os.Getenv("GHCR_USERNAME")
	ghcrToken := os.Getenv("GHCR_TOKEN")

	if ghcrUsername == "" || ghcrToken == "" {
		t.Skip("Skipping GHCR integration test - GHCR_USERNAME and GHCR_TOKEN environment variables not set")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()

	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err, "Failed to create Docker client")
	defer dockerClient.Close()

	// Create Docker manager
	cfg := testConfig()
	cfg.ImagePullTimeout = 90 * time.Second // Allow more time for private image pull
	manager := NewDockerManager(dockerClient, cfg)
	require.NotNil(t, manager)

	// Test image - using a private GHCR image
	privateImage := "ghcr.io/cyucelen/example-app:stable"

	t.Logf("Testing private GHCR image pull: %s", privateImage)

	// Create registry credentials
	creds := &model.RegistryCredentials{
		Username: ghcrUsername,
		Password: ghcrToken,
	}

	// Try to pull the private image with authentication
	t.Log("Pulling private GHCR image with authentication...")
	err = manager.EnsureImage(ctx, privateImage, creds)
	require.NoError(t, err, "Failed to pull private GHCR image with valid credentials")

	t.Logf("Successfully pulled private image: %s", privateImage)

	// Verify image exists locally
	images, err := dockerClient.ImageList(ctx, types.ImageListOptions{
		Filters: filters.NewArgs(filters.Arg("reference", privateImage)),
	})
	require.NoError(t, err)
	require.NotEmpty(t, images, "Image should exist locally after pull")

	t.Logf("Verified image exists locally with %d tag(s)", len(images))

	// Test creating a container from the private image
	t.Log("Creating container from private image...")
	container := &model.Container{
		ID:      uuid.New().String(),
		Image:   privateImage,
		Command: []string{"echo", "Hello from private GHCR image"},
		Labels:  map[string]string{model.LabelManaged: "true"},
	}

	dockerID, err := manager.CreateContainer(ctx, container, creds)
	require.NoError(t, err, "Failed to create container from private image")
	require.NotEmpty(t, dockerID)

	t.Logf("Created container from private image with Docker ID: %s", dockerID)

	// Cleanup
	defer func() {
		cleanupCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		_ = manager.RemoveContainer(cleanupCtx, dockerID)
		t.Logf("Cleaned up container: %s", dockerID)
	}()

	// Verify container was created successfully
	inspect, err := dockerClient.ContainerInspect(ctx, dockerID)
	require.NoError(t, err)
	assert.Equal(t, privateImage, inspect.Config.Image)

	t.Log("✓ Private GHCR image pull and container creation successful")
}

// T011b: Integration test - verify authentication failure with invalid credentials
func TestDockerManager_PrivateGHCRImagePull_InvalidCredentials_Integration(t *testing.T) {
	if testing.Short() {
		t.Skip("Skipping integration test in short mode")
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	// Create Docker client
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	require.NoError(t, err, "Failed to create Docker client")
	defer dockerClient.Close()

	// Create Docker manager
	manager := NewDockerManager(dockerClient, testConfig())
	require.NotNil(t, manager)

	// Test image - using a private GHCR image (should fail without valid creds)
	privateImage := "ghcr.io/cyucelen/example-app:stable"

	t.Logf("Testing private GHCR image pull with invalid credentials: %s", privateImage)

	// Create invalid credentials
	invalidCreds := &model.RegistryCredentials{
		Username: "invalid-user",
		Password: "invalid-token",
	}

	// Try to pull the private image with invalid authentication - should fail
	t.Log("Attempting to pull private image with invalid credentials...")
	err = manager.EnsureImage(ctx, privateImage, invalidCreds)
	
	// Should get an authentication error
	assert.Error(t, err, "Should fail to pull private image with invalid credentials")
	t.Logf("Expected authentication failure: %v", err)

	t.Log("✓ Authentication failure correctly detected")
}
