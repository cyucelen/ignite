package service

import (
	"context"
	"fmt"
	"testing"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/cyucelen/ignite/internal/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

// T039: Unit test for ReconcileContainers - no existing containers
func TestContainerService_ReconcileContainers_NoContainers(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 10}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Mock: Docker returns empty list
	mockDocker.On("ListIgniteContainers", mock.Anything).Return([]types.ContainerJSON{}, nil)

	// Execute
	ctx := context.Background()
	count, err := service.ReconcileContainers(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 0, count, "should reconcile 0 containers")
	mockDocker.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// T040: Unit test for ReconcileContainers - multiple valid containers
func TestContainerService_ReconcileContainers_MultipleContainers(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 10}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create sample Docker containers
	dockerContainers := []types.ContainerJSON{
		{
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
				Env:   []string{},
				Labels: map[string]string{
					"ignite.managed":    "true",
					"ignite.id":         "f47ac10b-58cc-4372-a567-0e02b2c3d479",
					"ignite.created-at": "2025-11-10T10:00:00Z",
					"ignite.image":      "alpine:latest",
					"ignite.version":    "1.0.0",
				},
			},
		},
		{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:      "docker456",
				Created: "2025-11-10T10:01:00Z",
				State: &types.ContainerState{
					Status:     "exited",
					Running:    false,
					StartedAt:  "2025-11-10T10:01:05Z",
					FinishedAt: "2025-11-10T10:05:00Z",
					ExitCode:   0,
				},
			},
			Config: &container.Config{
				Image: "nginx:latest",
				Cmd:   []string{"nginx"},
				Env:   []string{},
				Labels: map[string]string{
					"ignite.managed":    "true",
					"ignite.id":         "a1b2c3d4-e5f6-4789-a012-345678901234",
					"ignite.created-at": "2025-11-10T10:01:00Z",
					"ignite.image":      "nginx:latest",
					"ignite.version":    "1.0.0",
				},
			},
		},
	}

	// Mock: Docker returns 2 containers
	mockDocker.On("ListIgniteContainers", mock.Anything).Return(dockerContainers, nil)

	// Mock: Repository saves each container
	mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(c *model.Container) bool {
		return c.ID == "f47ac10b-58cc-4372-a567-0e02b2c3d479" && c.DockerID == "docker123"
	})).Return(nil).Once()

	mockRepo.On("Save", mock.Anything, mock.MatchedBy(func(c *model.Container) bool {
		return c.ID == "a1b2c3d4-e5f6-4789-a012-345678901234" && c.DockerID == "docker456"
	})).Return(nil).Once()

	// Execute
	ctx := context.Background()
	count, err := service.ReconcileContainers(ctx)

	// Assert
	require.NoError(t, err)
	assert.Equal(t, 2, count, "should reconcile 2 containers")
	mockDocker.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

// T041: Unit test for ReconcileContainers - Docker error
func TestContainerService_ReconcileContainers_DockerError(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 10}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Mock: Docker returns error
	dockerErr := fmt.Errorf("docker daemon unavailable")
	mockDocker.On("ListIgniteContainers", mock.Anything).Return([]types.ContainerJSON{}, dockerErr)

	// Execute
	ctx := context.Background()
	count, err := service.ReconcileContainers(ctx)

	// Assert
	require.Error(t, err)
	assert.Equal(t, 0, count)
	assert.Contains(t, err.Error(), "failed to list containers")
	mockDocker.AssertExpectations(t)
}

// T042: Unit test for ReconcileContainers - invalid container labels
func TestContainerService_ReconcileContainers_InvalidLabels(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 10}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create container with invalid labels (missing ignite.managed)
	dockerContainers := []types.ContainerJSON{
		{
			ContainerJSONBase: &types.ContainerJSONBase{
				ID:      "docker789",
				Created: "2025-11-10T10:00:00Z",
				State: &types.ContainerState{
					Status:  "running",
					Running: true,
				},
			},
			Config: &container.Config{
				Image: "alpine:latest",
				Cmd:   []string{"sleep", "3600"},
				Labels: map[string]string{
					// Missing "ignite.managed"
					"ignite.id":         "invalid-uuid",
					"ignite.created-at": "2025-11-10T10:00:00Z",
				},
			},
		},
	}

	// Mock: Docker returns container with invalid labels
	mockDocker.On("ListIgniteContainers", mock.Anything).Return(dockerContainers, nil)
	// Repository should NOT be called since reconstruction fails

	// Execute
	ctx := context.Background()
	count, err := service.ReconcileContainers(ctx)

	// Assert: Should skip invalid containers and continue
	require.NoError(t, err, "should skip invalid containers without failing")
	assert.Equal(t, 0, count, "should not count invalid containers")
	mockDocker.AssertExpectations(t)
	mockRepo.AssertNotCalled(t, "Save")
}

// T043: Unit test for ReconcileContainers - repository save error
func TestContainerService_ReconcileContainers_RepositorySaveError(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 10}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create valid Docker container
	dockerContainers := []types.ContainerJSON{
		{
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
				Env:   []string{},
				Labels: map[string]string{
					"ignite.managed":    "true",
					"ignite.id":         "f47ac10b-58cc-4372-a567-0e02b2c3d479",
					"ignite.created-at": "2025-11-10T10:00:00Z",
					"ignite.image":      "alpine:latest",
					"ignite.version":    "1.0.0",
				},
			},
		},
	}

	// Mock: Docker returns valid container
	mockDocker.On("ListIgniteContainers", mock.Anything).Return(dockerContainers, nil)

	// Mock: Repository returns error on save
	repoErr := fmt.Errorf("database connection lost")
	mockRepo.On("Save", mock.Anything, mock.Anything).Return(repoErr)

	// Execute
	ctx := context.Background()
	count, err := service.ReconcileContainers(ctx)

	// Assert: Should skip containers with save errors and continue
	require.NoError(t, err, "should skip save errors without failing")
	assert.Equal(t, 0, count, "should not count containers that failed to save")
	mockDocker.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}
