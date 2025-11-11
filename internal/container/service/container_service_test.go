package service

import (
	"context"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/cyucelen/ignite/internal/shared/config"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// MockDockerManager is a mock implementation of the DockerManager interface
type MockDockerManager struct {
	mock.Mock
}

func (m *MockDockerManager) CreateContainer(ctx context.Context, container *model.Container, creds *model.RegistryCredentials) (string, error) {
	args := m.Called(ctx, container, creds)
	return args.String(0), args.Error(1)
}

func (m *MockDockerManager) StartContainer(ctx context.Context, dockerID string) error {
	args := m.Called(ctx, dockerID)
	return args.Error(0)
}

func (m *MockDockerManager) StopContainer(ctx context.Context, dockerID string) error {
	args := m.Called(ctx, dockerID)
	return args.Error(0)
}

func (m *MockDockerManager) RemoveContainer(ctx context.Context, dockerID string) error {
	args := m.Called(ctx, dockerID)
	return args.Error(0)
}

func (m *MockDockerManager) InspectContainer(ctx context.Context, dockerID string) (*model.Container, error) {
	args := m.Called(ctx, dockerID)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Container), args.Error(1)
}

func (m *MockDockerManager) WatchEvents(ctx context.Context) (<-chan ContainerEvent, <-chan error) {
	args := m.Called(ctx)
	return args.Get(0).(<-chan ContainerEvent), args.Get(1).(<-chan error)
}

func (m *MockDockerManager) GetLogs(ctx context.Context, dockerID string, tail int, timestamps bool) ([]string, error) {
	args := m.Called(ctx, dockerID, tail, timestamps)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func (m *MockDockerManager) ListIgniteContainers(ctx context.Context) ([]types.ContainerJSON, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.ContainerJSON), args.Error(1)
}

// MockContainerRepository is a mock implementation of the ContainerRepository interface
type MockContainerRepository struct {
	mock.Mock
}

func (m *MockContainerRepository) Save(ctx context.Context, container *model.Container) error {
	args := m.Called(ctx, container)
	return args.Error(0)
}

func (m *MockContainerRepository) FindByID(ctx context.Context, id string) (*model.Container, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Container), args.Error(1)
}

func (m *MockContainerRepository) FindAll(ctx context.Context) ([]*model.Container, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Container), args.Error(1)
}

func (m *MockContainerRepository) Delete(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockContainerRepository) CountActive(ctx context.Context) (int, error) {
	args := m.Called(ctx)
	return args.Int(0), args.Error(1)
}

func TestContainerService_CreateContainer_ValidConfig(t *testing.T) {
	tests := []struct {
		name        string
		config      *model.ContainerConfig
		expectError bool
	}{
		{
			name: "valid config with all fields",
			config: &model.ContainerConfig{
				Image:   "nginx:latest",
				Command: []string{"nginx", "-g", "daemon off;"},
				Environment: map[string]string{
					"ENV": "test",
				},
				ResourceLimits: &model.ResourceLimits{
					MemoryMB: 512,
					CPUCores: 1.0,
				},
			},
			expectError: false,
		},
		{
			name: "valid config with minimal fields",
			config: &model.ContainerConfig{
				Image:   "alpine:latest",
				Command: []string{},
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDocker := new(MockDockerManager)
			mockRepo := new(MockContainerRepository)
			cfg := &config.Config{MaxContainers: 50}

			service := NewContainerService(mockDocker, mockRepo, cfg)

			// Setup mocks
			mockRepo.On("CountActive", mock.Anything).Return(0, nil)
			mockDocker.On("CreateContainer", mock.Anything, mock.AnythingOfType("*model.Container"), mock.Anything).
				Return("docker-id-123", nil)
			mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*model.Container")).Return(nil)
			mockDocker.On("StartContainer", mock.Anything, "docker-id-123").Return(nil)

			ctx := context.Background()
			container, err := service.CreateContainer(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, container)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, container)
				assert.Equal(t, tt.config.Image, container.Image)
				assert.Equal(t, model.Running, container.State)
			}

			mockDocker.AssertExpectations(t)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestContainerService_CreateContainer_InvalidImage(t *testing.T) {
	tests := []struct {
		name        string
		image       string
		expectError error
	}{
		{
			name:        "empty image",
			image:       "",
			expectError: model.ErrInvalidImage,
		},
		{
			name:        "invalid image format",
			image:       "not a valid image!!!",
			expectError: model.ErrInvalidImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDocker := new(MockDockerManager)
			mockRepo := new(MockContainerRepository)
			cfg := &config.Config{MaxContainers: 50}

			service := NewContainerService(mockDocker, mockRepo, cfg)

			config := &model.ContainerConfig{
				Image: tt.image,
			}

			ctx := context.Background()
			container, err := service.CreateContainer(ctx, config)

			assert.Error(t, err)
			assert.Nil(t, container)
			assert.ErrorIs(t, err, tt.expectError)
		})
	}
}

func TestContainerService_CreateContainer_LimitReached(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 2}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Mock repository to return count at limit
	mockRepo.On("CountActive", mock.Anything).Return(2, nil)

	config := &model.ContainerConfig{
		Image: "nginx:latest",
	}

	ctx := context.Background()
	container, err := service.CreateContainer(ctx, config)

	assert.Error(t, err)
	assert.Nil(t, container)
	assert.ErrorIs(t, err, model.ErrContainerLimitReached)
	mockRepo.AssertExpectations(t)
}

func TestContainerService_StartContainer_Success(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 50}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create a container in Creating state
	container := &model.Container{
		ID:       "container-123",
		DockerID: "docker-123",
		Image:    "nginx:latest",
		State:    model.Creating,
	}

	// Setup mocks
	mockRepo.On("FindByID", mock.Anything, "container-123").Return(container, nil)
	mockDocker.On("StartContainer", mock.Anything, "docker-123").Return(nil)
	mockRepo.On("Save", mock.Anything, container).Return(nil)

	ctx := context.Background()
	err := service.StartContainer(ctx, "container-123")

	assert.NoError(t, err)
	assert.Equal(t, model.Running, container.State)
	mockDocker.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestContainerService_StopContainer_Success(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 50}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create a container in Running state
	now := time.Now()
	container := &model.Container{
		ID:        "container-123",
		DockerID:  "docker-123",
		Image:     "nginx:latest",
		State:     model.Running,
		StartedAt: &now,
	}

	// Setup mocks
	mockRepo.On("FindByID", mock.Anything, "container-123").Return(container, nil)
	mockDocker.On("StopContainer", mock.Anything, "docker-123").Return(nil)
	mockRepo.On("Save", mock.Anything, container).Return(nil)

	ctx := context.Background()
	err := service.StopContainer(ctx, "container-123")

	assert.NoError(t, err)
	assert.Equal(t, model.Stopped, container.State)
	mockDocker.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestContainerService_RemoveContainer_Success(t *testing.T) {
	tests := []struct {
		name           string
		initialState   model.ContainerState
		shouldCallStop bool
	}{
		{
			name:           "remove exited container",
			initialState:   model.Exited,
			shouldCallStop: false,
		},
		{
			name:           "remove stopped container",
			initialState:   model.Stopped,
			shouldCallStop: false,
		},
		{
			name:           "remove running container (should stop first)",
			initialState:   model.Running,
			shouldCallStop: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDocker := new(MockDockerManager)
			mockRepo := new(MockContainerRepository)
			cfg := &config.Config{MaxContainers: 50}

			service := NewContainerService(mockDocker, mockRepo, cfg)

			container := &model.Container{
				ID:       "container-123",
				DockerID: "docker-123",
				Image:    "nginx:latest",
				State:    tt.initialState,
			}

			// Setup mocks
			mockRepo.On("FindByID", mock.Anything, "container-123").Return(container, nil)

			if tt.shouldCallStop {
				mockDocker.On("StopContainer", mock.Anything, "docker-123").Return(nil)
			}

			mockDocker.On("RemoveContainer", mock.Anything, "docker-123").Return(nil)
			mockRepo.On("Save", mock.Anything, container).Return(nil)

			ctx := context.Background()
			err := service.RemoveContainer(ctx, "container-123")

			assert.NoError(t, err)
			assert.Equal(t, model.Removed, container.State)
			mockDocker.AssertExpectations(t)
			mockRepo.AssertExpectations(t)
		})
	}
}

func TestContainerService_AutomaticCleanup(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{
		MaxContainers:    50,
		ContainerTimeout: 5, // 5 seconds timeout
	}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create containers that should be cleaned up
	exitTime := time.Now().Add(-10 * time.Minute) // Exited 10 minutes ago
	exitCode := 0

	containers := []*model.Container{
		{
			ID:       "container-1",
			DockerID: "docker-1",
			State:    model.Exited,
			ExitedAt: &exitTime,
			ExitCode: &exitCode,
		},
		{
			ID:       "container-2",
			DockerID: "docker-2",
			State:    model.Running, // Should not be cleaned
		},
	}

	// Setup mocks
	mockRepo.On("FindAll", mock.Anything).Return(containers, nil)
	mockDocker.On("RemoveContainer", mock.Anything, "docker-1").Return(nil)
	mockRepo.On("Save", mock.Anything, containers[0]).Return(nil)

	ctx := context.Background()
	err := service.CleanupExitedContainers(ctx)

	assert.NoError(t, err)
	assert.Equal(t, model.Removed, containers[0].State)
	assert.Equal(t, model.Running, containers[1].State) // Should not change
	mockDocker.AssertExpectations(t)
	mockRepo.AssertExpectations(t)
}

func TestContainerService_HandleContainerEvents(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 50}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create event channels
	eventChan := make(chan ContainerEvent, 1)
	errChan := make(chan error, 1)

	mockDocker.On("WatchEvents", mock.Anything).Return(
		(<-chan ContainerEvent)(eventChan),
		(<-chan error)(errChan),
	)

	// Create a container to receive events for
	container := &model.Container{
		ID:       "container-123",
		DockerID: "docker-123",
		State:    model.Running,
	}

	mockRepo.On("FindAll", mock.Anything).Return([]*model.Container{container}, nil)
	mockRepo.On("Save", mock.Anything, container).Return(nil)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Start event handler
	go service.HandleContainerEvents(ctx)

	// Send a die event
	exitCode := 0
	eventChan <- ContainerEvent{
		DockerID:  "docker-123",
		EventType: "die",
		ExitCode:  &exitCode,
		Timestamp: time.Now(),
	}

	// Give time for event to be processed
	time.Sleep(100 * time.Millisecond)
	cancel()

	// Verify container state was updated
	mockRepo.AssertCalled(t, "Save", mock.Anything, container)
}

// T037: Test GetContainer success
func TestContainerService_GetContainer_Success(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 50}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create a test container
	now := time.Now()
	expectedContainer := &model.Container{
		ID:        "container-123",
		DockerID:  "docker-123",
		Image:     "nginx:latest",
		State:     model.Running,
		CreatedAt: now,
		StartedAt: &now,
	}

	// Setup mock
	mockRepo.On("FindByID", mock.Anything, "container-123").Return(expectedContainer, nil)

	ctx := context.Background()
	container, err := service.GetContainer(ctx, "container-123")

	assert.NoError(t, err)
	assert.NotNil(t, container)
	assert.Equal(t, expectedContainer.ID, container.ID)
	assert.Equal(t, expectedContainer.DockerID, container.DockerID)
	assert.Equal(t, expectedContainer.Image, container.Image)
	assert.Equal(t, expectedContainer.State, container.State)

	mockRepo.AssertExpectations(t)
}

// T037: Test GetContainer not found
func TestContainerService_GetContainer_NotFound(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 50}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Setup mock to return not found error
	mockRepo.On("FindByID", mock.Anything, "nonexistent-id").Return(nil, model.ErrContainerNotFound)

	ctx := context.Background()
	container, err := service.GetContainer(ctx, "nonexistent-id")

	assert.Error(t, err)
	assert.Nil(t, container)
	assert.ErrorIs(t, err, model.ErrContainerNotFound)

	mockRepo.AssertExpectations(t)
}

// T038: Test ListContainers success
func TestContainerService_ListContainers_Success(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 50}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Create test containers
	now := time.Now()
	expectedContainers := []*model.Container{
		{
			ID:        "container-1",
			DockerID:  "docker-1",
			Image:     "nginx:latest",
			State:     model.Running,
			CreatedAt: now,
			StartedAt: &now,
		},
		{
			ID:        "container-2",
			DockerID:  "docker-2",
			Image:     "alpine:latest",
			State:     model.Stopped,
			CreatedAt: now,
		},
	}

	// Setup mock
	mockRepo.On("FindAll", mock.Anything).Return(expectedContainers, nil)

	ctx := context.Background()
	containers, err := service.ListContainers(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, containers)
	assert.Len(t, containers, 2)
	assert.Equal(t, expectedContainers[0].ID, containers[0].ID)
	assert.Equal(t, expectedContainers[1].ID, containers[1].ID)

	mockRepo.AssertExpectations(t)
}

// T038: Test ListContainers empty
func TestContainerService_ListContainers_Empty(t *testing.T) {
	mockDocker := new(MockDockerManager)
	mockRepo := new(MockContainerRepository)
	cfg := &config.Config{MaxContainers: 50}

	service := NewContainerService(mockDocker, mockRepo, cfg)

	// Setup mock to return empty list
	mockRepo.On("FindAll", mock.Anything).Return([]*model.Container{}, nil)

	ctx := context.Background()
	containers, err := service.ListContainers(ctx)

	assert.NoError(t, err)
	assert.NotNil(t, containers)
	assert.Len(t, containers, 0)

	mockRepo.AssertExpectations(t)
}

// T022: Test CreateContainer with registry credentials
func TestContainerService_CreateContainer_WithRegistryAuth(t *testing.T) {
	tests := []struct {
		name         string
		config       *model.ContainerConfig
		expectCreds  *model.RegistryCredentials
		expectError  bool
	}{
		{
			name: "private image with GHCR credentials",
			config: &model.ContainerConfig{
				Image:   "ghcr.io/owner/private-image:latest",
				Command: []string{"echo", "hello"},
				RegistryAuth: &model.RegistryCredentials{
					Username: "testuser",
					Password: "ghp_testtoken123",
				},
			},
			expectCreds: &model.RegistryCredentials{
				Username: "testuser",
				Password: "ghp_testtoken123",
			},
			expectError: false,
		},
		{
			name: "public image without credentials",
			config: &model.ContainerConfig{
				Image:        "nginx:latest",
				Command:      []string{"nginx"},
				RegistryAuth: nil,
			},
			expectCreds: nil,
			expectError: false,
		},
		{
			name: "private image with empty credentials",
			config: &model.ContainerConfig{
				Image:   "ghcr.io/owner/image:tag",
				Command: []string{"./app"},
				RegistryAuth: &model.RegistryCredentials{
					Username: "",
					Password: "",
				},
			},
			expectCreds: &model.RegistryCredentials{
				Username: "",
				Password: "",
			},
			expectError: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockDocker := new(MockDockerManager)
			mockRepo := new(MockContainerRepository)
			cfg := &config.Config{MaxContainers: 50}

			service := NewContainerService(mockDocker, mockRepo, cfg)

			// Setup mocks
			mockRepo.On("CountActive", mock.Anything).Return(0, nil)
			
			// Verify that credentials are passed through to Docker manager
			mockDocker.On("CreateContainer", 
				mock.Anything, 
				mock.AnythingOfType("*model.Container"),
				mock.MatchedBy(func(creds *model.RegistryCredentials) bool {
					// Verify credentials match expected
					if tt.expectCreds == nil {
						return creds == nil
					}
					if creds == nil {
						return false
					}
					return creds.Username == tt.expectCreds.Username && 
					       creds.Password == tt.expectCreds.Password
				}),
			).Return("docker-id-123", nil)
			
			mockRepo.On("Save", mock.Anything, mock.AnythingOfType("*model.Container")).Return(nil)
			mockDocker.On("StartContainer", mock.Anything, "docker-id-123").Return(nil)

			ctx := context.Background()
			container, err := service.CreateContainer(ctx, tt.config)

			if tt.expectError {
				assert.Error(t, err)
				assert.Nil(t, container)
			} else {
				assert.NoError(t, err)
				assert.NotNil(t, container)
				assert.Equal(t, tt.config.Image, container.Image)
				assert.Equal(t, model.Running, container.State)
			}

			mockDocker.AssertExpectations(t)
			mockRepo.AssertExpectations(t)
		})
	}
}
