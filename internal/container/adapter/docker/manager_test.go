package docker

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/image"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/cyucelen/ignite/internal/shared/config"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const testDockerID = "docker-id-123"

// MockDockerClient is a mock implementation of the Docker client interface
type MockDockerClient struct {
	mock.Mock
}

func (m *MockDockerClient) ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *networktypes.NetworkingConfig, platformConfig *v1.Platform, containerName string) (container.CreateResponse, error) {
	args := m.Called(ctx, config, hostConfig, networkingConfig, platformConfig, containerName)
	return args.Get(0).(container.CreateResponse), args.Error(1)
}

func (m *MockDockerClient) ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error {
	args := m.Called(ctx, containerID, options)
	return args.Error(0)
}

func (m *MockDockerClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	args := m.Called(ctx, containerID)
	return args.Get(0).(types.ContainerJSON), args.Error(1)
}

func (m *MockDockerClient) Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error) {
	args := m.Called(ctx, options)
	return args.Get(0).(<-chan events.Message), args.Get(1).(<-chan error)
}

func (m *MockDockerClient) Close() error {
	args := m.Called()
	return args.Error(0)
}

func (m *MockDockerClient) Ping(ctx context.Context) (types.Ping, error) {
	args := m.Called(ctx)
	return args.Get(0).(types.Ping), args.Error(1)
}

func (m *MockDockerClient) ImagePull(ctx context.Context, refStr string, options types.ImagePullOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, refStr, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

func (m *MockDockerClient) ImageList(ctx context.Context, options types.ImageListOptions) ([]image.Summary, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]image.Summary), args.Error(1)
}

func (m *MockDockerClient) ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error) {
	args := m.Called(ctx, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]types.Container), args.Error(1)
}

func (m *MockDockerClient) ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error) {
	args := m.Called(ctx, containerID, options)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(io.ReadCloser), args.Error(1)
}

// Helper function to create test config
func testConfig() *config.Config {
	return &config.Config{
		ImagePullTimeout: 5 * time.Minute,
		MaxContainers:    50,
	}
}

func TestDockerManager_CreateContainer_Success(t *testing.T) {
	tests := []struct {
		name      string
		container *model.Container
		dockerID  string
	}{
		{
			name: "create container with all options",
			container: &model.Container{
				ID:      "container-123",
				Image:   "nginx:latest",
				Command: []string{"nginx", "-g", "daemon off;"},
				EnvironmentVariables: map[string]string{
					"ENV": "test",
				},
				ResourceLimits: &model.ResourceLimits{
					MemoryMB: 512,
					CPUCores: 1.0,
				},
				Labels: map[string]string{
					"ignite.managed": "true",
				},
			},
			dockerID: "docker-id-123",
		},
		{
			name: "create container with minimal options",
			container: &model.Container{
				ID:    "container-456",
				Image: "alpine:latest",
			},
			dockerID: "docker-id-456",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			manager := NewDockerManager(mockClient, testConfig())

			// Mock ImageList to simulate cached image
			mockClient.On("ImageList", mock.Anything, mock.Anything).Return([]image.Summary{{ID: "image-123"}}, nil)

			// Setup mock expectations
			mockClient.On("ContainerCreate",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(container.CreateResponse{
				ID: tt.dockerID,
			}, nil)

			ctx := context.Background()
			dockerID, err := manager.CreateContainer(ctx, tt.container, nil)

			assert.NoError(t, err)
			assert.Equal(t, tt.dockerID, dockerID)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDockerManager_CreateContainer_Error(t *testing.T) {
	tests := []struct {
		name        string
		mockError   error
		expectError bool
	}{
		{
			name:        "docker client error",
			mockError:   errors.New("docker daemon unavailable"),
			expectError: true,
		},
		{
			name:        "image not found",
			mockError:   errors.New("image not found"),
			expectError: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			manager := NewDockerManager(mockClient, testConfig())

			// Mock ImageList to simulate cached image
			mockClient.On("ImageList", mock.Anything, mock.Anything).Return([]image.Summary{{ID: "image-123"}}, nil)

			cont := &model.Container{
				ID:    "container-123",
				Image: "invalid:image",
			}

			mockClient.On("ContainerCreate",
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
				mock.Anything,
			).Return(container.CreateResponse{}, tt.mockError)

			ctx := context.Background()
			dockerID, err := manager.CreateContainer(ctx, cont, nil)

			assert.Error(t, err)
			assert.Empty(t, dockerID)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDockerManager_StartContainer_Success(t *testing.T) {
	mockClient := new(MockDockerClient)
	manager := NewDockerManager(mockClient, testConfig())

	dockerID := testDockerID

	mockClient.On("ContainerStart",
		mock.Anything,
		dockerID,
		mock.Anything,
	).Return(nil)

	ctx := context.Background()
	err := manager.StartContainer(ctx, dockerID)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDockerManager_StartContainer_Error(t *testing.T) {
	tests := []struct {
		name      string
		mockError error
	}{
		{
			name:      "container not found",
			mockError: errors.New("container not found"),
		},
		{
			name:      "container already started",
			mockError: errors.New("container already started"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			manager := NewDockerManager(mockClient, testConfig())

			dockerID := testDockerID

			mockClient.On("ContainerStart",
				mock.Anything,
				dockerID,
				mock.Anything,
			).Return(tt.mockError)

			ctx := context.Background()
			err := manager.StartContainer(ctx, dockerID)

			assert.Error(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDockerManager_StopContainer_Success(t *testing.T) {
	mockClient := new(MockDockerClient)
	manager := NewDockerManager(mockClient, testConfig())

	dockerID := testDockerID

	mockClient.On("ContainerStop",
		mock.Anything,
		dockerID,
		mock.Anything,
	).Return(nil)

	ctx := context.Background()
	err := manager.StopContainer(ctx, dockerID)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

func TestDockerManager_StopContainer_Error(t *testing.T) {
	tests := []struct {
		name      string
		mockError error
	}{
		{
			name:      "container not found",
			mockError: errors.New("container not found"),
		},
		{
			name:      "container already stopped",
			mockError: errors.New("container not running"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			manager := NewDockerManager(mockClient, testConfig())

			dockerID := testDockerID

			mockClient.On("ContainerStop",
				mock.Anything,
				dockerID,
				mock.Anything,
			).Return(tt.mockError)

			ctx := context.Background()
			err := manager.StopContainer(ctx, dockerID)

			assert.Error(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDockerManager_RemoveContainer_Success(t *testing.T) {
	tests := []struct {
		name         string
		forceRemove  bool
		removeVolume bool
	}{
		{
			name:         "remove with force",
			forceRemove:  true,
			removeVolume: true,
		},
		{
			name:         "remove without force",
			forceRemove:  false,
			removeVolume: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			manager := NewDockerManager(mockClient, testConfig())

			dockerID := testDockerID

			mockClient.On("ContainerRemove",
				mock.Anything,
				dockerID,
				mock.Anything,
			).Return(nil)

			ctx := context.Background()
			err := manager.RemoveContainer(ctx, dockerID)

			assert.NoError(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDockerManager_RemoveContainer_Error(t *testing.T) {
	tests := []struct {
		name      string
		mockError error
	}{
		{
			name:      "container not found",
			mockError: errors.New("container not found"),
		},
		{
			name:      "container still running",
			mockError: errors.New("cannot remove running container"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			manager := NewDockerManager(mockClient, testConfig())

			dockerID := testDockerID

			mockClient.On("ContainerRemove",
				mock.Anything,
				dockerID,
				mock.Anything,
			).Return(tt.mockError)

			ctx := context.Background()
			err := manager.RemoveContainer(ctx, dockerID)

			assert.Error(t, err)
			mockClient.AssertExpectations(t)
		})
	}
}

func TestDockerManager_WatchEvents_Success(t *testing.T) {
	mockClient := new(MockDockerClient)
	manager := NewDockerManager(mockClient, testConfig())

	// Create event channels
	eventChan := make(chan events.Message, 2)
	errChan := make(chan error, 1)

	mockClient.On("Events",
		mock.Anything,
		mock.Anything,
	).Return(
		(<-chan events.Message)(eventChan),
		(<-chan error)(errChan),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	outEventChan, _ := manager.WatchEvents(ctx)

	// Send test events
	exitCode := int64(0)
	eventChan <- events.Message{
		Type:   "container",
		Action: events.Action("die"),
		Actor: events.Actor{
			ID: "docker-id-123",
			Attributes: map[string]string{
				"exitCode": "0",
			},
		},
		Time: time.Now().Unix(),
	}

	// Read from output channel
	select {
	case event := <-outEventChan:
		assert.Equal(t, "docker-id-123", event.DockerID)
		assert.Equal(t, "die", event.EventType)
		assert.NotNil(t, event.ExitCode)
		assert.Equal(t, int(exitCode), *event.ExitCode)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for event")
	}

	mockClient.AssertExpectations(t)
}

func TestDockerManager_WatchEvents_ErrorHandling(t *testing.T) {
	mockClient := new(MockDockerClient)
	manager := NewDockerManager(mockClient, testConfig())

	eventChan := make(chan events.Message)
	errChan := make(chan error, 1)

	mockClient.On("Events",
		mock.Anything,
		mock.Anything,
	).Return(
		(<-chan events.Message)(eventChan),
		(<-chan error)(errChan),
	)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	_, outErrChan := manager.WatchEvents(ctx)
	_ = outErrChan // Assign to blank identifier to avoid unused variable error

	// Send error
	testErr := errors.New("docker event stream error")
	errChan <- testErr

	// Read error from output channel
	select {
	case err := <-outErrChan:
		assert.Error(t, err)
		assert.Equal(t, testErr, err)
	case <-time.After(1 * time.Second):
		t.Fatal("timeout waiting for error")
	}

	mockClient.AssertExpectations(t)
}

func TestDockerManager_WatchEvents_MultipleEventTypes(t *testing.T) {
	tests := []struct {
		name           string
		eventAction    string
		expectedType   string
		exitCode       string
		expectExitCode bool
	}{
		{
			name:           "die event with exit code",
			eventAction:    "die",
			expectedType:   "die",
			exitCode:       "0",
			expectExitCode: true,
		},
		{
			name:           "start event",
			eventAction:    "start",
			expectedType:   "start",
			exitCode:       "",
			expectExitCode: false,
		},
		{
			name:           "stop event",
			eventAction:    "stop",
			expectedType:   "stop",
			exitCode:       "",
			expectExitCode: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			manager := NewDockerManager(mockClient, testConfig())

			eventChan := make(chan events.Message, 1)
			errChan := make(chan error)

			mockClient.On("Events",
				mock.Anything,
				mock.Anything,
			).Return(
				(<-chan events.Message)(eventChan),
				(<-chan error)(errChan),
			)

			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()

			outEventChan, _ := manager.WatchEvents(ctx)

			// Send test event
			attributes := map[string]string{}
			if tt.exitCode != "" {
				attributes["exitCode"] = tt.exitCode
			}

			eventChan <- events.Message{
				Type:   "container",
				Action: events.Action(tt.eventAction),
				Actor: events.Actor{
					ID:         "docker-id-123",
					Attributes: attributes,
				},
				Time: time.Now().Unix(),
			}

			// Read from output channel
			select {
			case event := <-outEventChan:
				assert.Equal(t, "docker-id-123", event.DockerID)
				assert.Equal(t, tt.expectedType, event.EventType)
				if tt.expectExitCode {
					assert.NotNil(t, event.ExitCode)
				}
			case <-time.After(1 * time.Second):
				t.Fatal("timeout waiting for event")
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// T051: Test Docker daemon connectivity check - Success
func TestDockerManager_Ping_Success(t *testing.T) {
	mockClient := new(MockDockerClient)
	manager := NewDockerManager(mockClient, testConfig())

	mockClient.On("Ping",
		mock.Anything,
	).Return(types.Ping{}, nil)

	ctx := context.Background()
	err := manager.Ping(ctx)

	assert.NoError(t, err)
	mockClient.AssertExpectations(t)
}

// T051: Test Docker daemon connectivity check - Failure
func TestDockerManager_Ping_Failure(t *testing.T) {
	tests := []struct {
		name      string
		mockError error
	}{
		{
			name:      "docker daemon unavailable",
			mockError: errors.New("cannot connect to Docker daemon"),
		},
		{
			name:      "docker socket not found",
			mockError: errors.New("docker socket not found"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			mockClient := new(MockDockerClient)
			manager := NewDockerManager(mockClient, testConfig())

			mockClient.On("Ping",
				mock.Anything,
			).Return(types.Ping{}, tt.mockError)

			ctx := context.Background()
			err := manager.Ping(ctx)

			assert.Error(t, err)
			assert.Equal(t, tt.mockError, err)
			mockClient.AssertExpectations(t)
		})
	}
}
// T002: Unit tests for extractRegistryHost (FOUNDATIONAL - Phase 2)
func TestExtractRegistryHost(t *testing.T) {
	tests := []struct {
		name      string
		imageName string
		expected  string
	}{
		{
			name:      "GHCR image",
			imageName: "ghcr.io/owner/image:tag",
			expected:  "ghcr.io",
		},
		{
			name:      "Docker Hub implicit (no registry)",
			imageName: "nginx:latest",
			expected:  "",
		},
		{
			name:      "Docker Hub explicit",
			imageName: "docker.io/library/nginx:latest",
			expected:  "docker.io",
		},
		{
			name:      "AWS ECR",
			imageName: "123456789.dkr.ecr.us-east-1.amazonaws.com/image:tag",
			expected:  "123456789.dkr.ecr.us-east-1.amazonaws.com",
		},
		{
			name:      "Quay.io",
			imageName: "quay.io/owner/image",
			expected:  "quay.io",
		},
		{
			name:      "Local registry with port",
			imageName: "localhost:5000/myimage:tag",
			expected:  "localhost:5000",
		},
		{
			name:      "Image with digest",
			imageName: "ghcr.io/owner/image@sha256:abc123",
			expected:  "ghcr.io",
		},
		{
			name:      "Simple image name",
			imageName: "alpine",
			expected:  "",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := extractRegistryHost(tt.imageName)
			assert.Equal(t, tt.expected, got, "extractRegistryHost(%q) = %q, want %q", tt.imageName, got, tt.expected)
		})
	}
}

// T004: Unit tests for redactPassword (FOUNDATIONAL - Phase 2)
func TestRedactPassword(t *testing.T) {
	tests := []struct {
		name     string
		msg      string
		password string
		expected string
	}{
		{
			name:     "redact password in error message",
			msg:      "authentication failed with token ghp_abc123xyz",
			password: "ghp_abc123xyz",
			expected: "authentication failed with token [REDACTED]",
		},
		{
			name:     "redact password multiple occurrences",
			msg:      "token ghp_secret used for ghp_secret auth",
			password: "ghp_secret",
			expected: "token [REDACTED] used for [REDACTED] auth",
		},
		{
			name:     "empty password (no redaction)",
			msg:      "some error message with ghp_token",
			password: "",
			expected: "some error message with ghp_token",
		},
		{
			name:     "password not in message",
			msg:      "authentication failed",
			password: "ghp_notfound",
			expected: "authentication failed",
		},
		{
			name:     "empty message",
			msg:      "",
			password: "ghp_token",
			expected: "",
		},
		{
			name:     "password at start",
			msg:      "ghp_token is invalid",
			password: "ghp_token",
			expected: "[REDACTED] is invalid",
		},
		{
			name:     "password at end",
			msg:      "invalid token: ghp_token",
			password: "ghp_token",
			expected: "invalid token: [REDACTED]",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := redactPassword(tt.msg, tt.password)
			assert.Equal(t, tt.expected, got, "redactPassword(%q, %q) = %q, want %q", tt.msg, tt.password, got, tt.expected)
		})
	}
}

// T008: Test for encodeAuthCredentials() method
func TestDockerManager_EncodeAuthCredentials(t *testing.T) {
	mgr := NewDockerManager(nil, testConfig())

	tests := []struct {
		name      string
		imageName string
		creds     *model.RegistryCredentials
		wantErr   bool
		validate  func(*testing.T, string)
	}{
		{
			name:      "valid GHCR credentials",
			imageName: "ghcr.io/owner/image:tag",
			creds: &model.RegistryCredentials{
				Username: "testuser",
				Password: "ghp_testtoken123",
			},
			wantErr: false,
			validate: func(t *testing.T, encoded string) {
				assert.NotEmpty(t, encoded)
				// Encoded auth should be base64
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				require.NoError(t, err)
				// Should be valid JSON
				var authConfig map[string]interface{}
				err = json.Unmarshal(decoded, &authConfig)
				require.NoError(t, err)
				assert.Equal(t, "testuser", authConfig["username"])
				assert.Equal(t, "ghp_testtoken123", authConfig["password"])
				assert.Equal(t, "ghcr.io", authConfig["serveraddress"])
			},
		},
		{
			name:      "nil credentials",
			imageName: "nginx:latest",
			creds:     nil,
			wantErr:   false,
			validate: func(t *testing.T, encoded string) {
				assert.Empty(t, encoded)
			},
		},
		{
			name:      "Docker Hub image with credentials",
			imageName: "nginx:latest",
			creds: &model.RegistryCredentials{
				Username: "dockeruser",
				Password: "dockerpass",
			},
			wantErr: false,
			validate: func(t *testing.T, encoded string) {
				assert.NotEmpty(t, encoded)
				decoded, err := base64.StdEncoding.DecodeString(encoded)
				require.NoError(t, err)
				var authConfig map[string]interface{}
				err = json.Unmarshal(decoded, &authConfig)
				require.NoError(t, err)
				// Docker Hub should have empty or missing serveraddress
				serverAddr, ok := authConfig["serveraddress"]
				if ok {
					assert.Equal(t, "", serverAddr)
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			encoded, err := mgr.encodeAuthCredentials(tt.imageName, tt.creds)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
				if tt.validate != nil {
					tt.validate(t, encoded)
				}
			}
		})
	}
}

// T009: Test for EnsureImage() with credentials
func TestDockerManager_EnsureImage_WithCredentials(t *testing.T) {
	mockClient := new(MockDockerClient)
	mgr := NewDockerManager(mockClient, testConfig())

	tests := []struct {
		name      string
		imageName string
		creds     *model.RegistryCredentials
		wantErr   bool
	}{
		{
			name:      "GHCR image with valid credentials",
			imageName: "ghcr.io/owner/private-image:latest",
			creds: &model.RegistryCredentials{
				Username: "testuser",
				Password: "ghp_testtoken123",
			},
			wantErr: false,
		},
		{
			name:      "public image without credentials",
			imageName: "alpine:latest",
			creds:     nil,
			wantErr:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Mock ImageList to simulate image exists
			mockClient.On("ImageList", mock.Anything, mock.Anything).
				Return([]image.Summary{{ID: "test-image-id"}}, nil).
				Once()

			ctx := context.Background()
			err := mgr.EnsureImage(ctx, tt.imageName, tt.creds)

			if tt.wantErr {
				assert.Error(t, err)
			} else {
				assert.NoError(t, err)
			}

			mockClient.AssertExpectations(t)
		})
	}
}

// T010: Test for pullImage() with authentication
func TestDockerManager_PullImage_WithAuthentication(t *testing.T) {
	t.Skip("Signature validation only - covered by integration tests (T011)")
	// This test was created to validate that pullImage() accepts credentials parameter.
	// The actual functionality is tested in:
	// - TestDockerManager_PrivateGHCRImagePull_Integration (integration test with real Docker)
	// - TestDockerManager_EncodeAuthCredentials (unit test for auth encoding)
}
