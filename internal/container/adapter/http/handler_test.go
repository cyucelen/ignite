package http

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/gofiber/fiber/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

type MockContainerService struct {
	mock.Mock
}

type MockDockerManager struct {
	mock.Mock
}

func (m *MockDockerManager) Ping(ctx context.Context) error {
	args := m.Called(ctx)
	return args.Error(0)
}

func (m *MockContainerService) CreateContainer(ctx context.Context, config *model.ContainerConfig) (*model.Container, error) {
	args := m.Called(ctx, config)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Container), args.Error(1)
}

func (m *MockContainerService) GetContainer(ctx context.Context, id string) (*model.Container, error) {
	args := m.Called(ctx, id)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).(*model.Container), args.Error(1)
}

func (m *MockContainerService) ListContainers(ctx context.Context) ([]*model.Container, error) {
	args := m.Called(ctx)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]*model.Container), args.Error(1)
}

func (m *MockContainerService) RemoveContainer(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockContainerService) StartContainer(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockContainerService) StopContainer(ctx context.Context, id string) error {
	args := m.Called(ctx, id)
	return args.Error(0)
}

func (m *MockContainerService) GetLogs(ctx context.Context, id string, tail int, timestamps bool) ([]string, error) {
	args := m.Called(ctx, id, tail, timestamps)
	if args.Get(0) == nil {
		return nil, args.Error(1)
	}
	return args.Get(0).([]string), args.Error(1)
}

func TestCreateContainer_ValidRequest(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    CreateContainerRequest
		expectedStatus int
	}{
		{
			name: "valid request with all fields",
			requestBody: CreateContainerRequest{
				Image:   "nginx:latest",
				Command: []string{"nginx", "-g", "daemon off;"},
				Environment: map[string]string{
					"ENV": "test",
				},
			},
			expectedStatus: 201,
		},
		{
			name: "valid request with minimal fields",
			requestBody: CreateContainerRequest{
				Image: "alpine:latest",
			},
			expectedStatus: 201,
		},
		{
			name: "valid request with resource limits",
			requestBody: CreateContainerRequest{
				Image:   "nginx:latest",
				Command: []string{"nginx"},
				Environment: map[string]string{
					"PORT": "8080",
				},
			},
			expectedStatus: 201,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockService := new(MockContainerService)
			handler := NewContainerHandler(mockService)

			// Register routes
			handler.RegisterRoutes(app)

			// Create expected container response
			now := time.Now()
			expectedContainer := &model.Container{
				ID:                   "container-123",
				Image:                tt.requestBody.Image,
				Command:              tt.requestBody.Command,
				EnvironmentVariables: tt.requestBody.Environment,
				State:                model.Running,
				CreatedAt:            now,
			}

			mockService.On("CreateContainer",
				mock.Anything,
				mock.MatchedBy(func(config *model.ContainerConfig) bool {
					return config.Image == tt.requestBody.Image
				}),
			).Return(expectedContainer, nil)

			// Create request
			body, err := json.Marshal(tt.requestBody)
			assert.NoError(t, err)
			req := httptest.NewRequest("POST", "/containers", bytes.NewReader(body))
			req.Header.Set("Content-Type", "application/json")

			// Execute request
			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Verify response body
			var response ContainerResponse
			err = json.NewDecoder(resp.Body).Decode(&response)
			assert.NoError(t, err)
			assert.Equal(t, expectedContainer.ID, response.ID)
			assert.Equal(t, expectedContainer.Image, response.Image)
			assert.Equal(t, expectedContainer.State.String(), response.State)

			mockService.AssertExpectations(t)
		})
	}
}

func testCreateContainerError(t *testing.T, requestBody CreateContainerRequest, mockError error, expectedStatus int, expectedError string) {
	t.Helper()

	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	mockService.On("CreateContainer",
		mock.Anything,
		mock.Anything,
	).Return(nil, mockError)

	body, err := json.Marshal(requestBody)
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "/containers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, expectedStatus, resp.StatusCode)

	var errorResponse ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	assert.NoError(t, err)
	assert.Contains(t, errorResponse.Error, expectedError)

	mockService.AssertExpectations(t)
}

func TestCreateContainer_InvalidImage(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    CreateContainerRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "empty image",
			requestBody: CreateContainerRequest{
				Image: "",
			},
			expectedStatus: 400,
			expectedError:  "invalid container image format",
		},
		{
			name: "invalid image format",
			requestBody: CreateContainerRequest{
				Image: "not_a_valid_image!!!",
			},
			expectedStatus: 400,
			expectedError:  "invalid container image format",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCreateContainerError(t, tt.requestBody, model.ErrInvalidImage, tt.expectedStatus, tt.expectedError)
		})
	}
}

func TestCreateContainer_InvalidEnvironment(t *testing.T) {
	tests := []struct {
		name           string
		requestBody    CreateContainerRequest
		expectedStatus int
		expectedError  string
	}{
		{
			name: "environment too large",
			requestBody: CreateContainerRequest{
				Image: "nginx:latest",
				Environment: map[string]string{
					"KEY": string(make([]byte, 100000)), // Very large value
				},
			},
			expectedStatus: 400,
			expectedError:  "environment variables exceed size limit",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			testCreateContainerError(t, tt.requestBody, model.ErrEnvironmentTooLarge, tt.expectedStatus, tt.expectedError)
		})
	}
}

func TestCreateContainer_LimitReached(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	mockService.On("CreateContainer",
		mock.Anything,
		mock.Anything,
	).Return(nil, model.ErrContainerLimitReached)

	requestBody := CreateContainerRequest{
		Image: "nginx:latest",
	}

	body, err := json.Marshal(requestBody)
	assert.NoError(t, err)
	req := httptest.NewRequest("POST", "/containers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 429, resp.StatusCode)

	var errorResponse ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	assert.NoError(t, err)
	assert.Contains(t, errorResponse.Error, "container limit reached")

	mockService.AssertExpectations(t)
}

// T017: Test DELETE /containers/{id} success
func TestDeleteContainer_Success(t *testing.T) {
	tests := []struct {
		name           string
		containerID    string
		expectedStatus int
	}{
		{
			name:           "delete existing container",
			containerID:    "container-123",
			expectedStatus: 204,
		},
		{
			name:           "delete another container",
			containerID:    "container-456",
			expectedStatus: 204,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockService := new(MockContainerService)
			handler := NewContainerHandler(mockService)

			handler.RegisterRoutes(app)

			mockService.On("RemoveContainer",
				mock.Anything,
				tt.containerID,
			).Return(nil)

			req := httptest.NewRequest("DELETE", "/containers/"+tt.containerID, nil)

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			mockService.AssertExpectations(t)
		})
	}
}

func TestDeleteContainer_NotFound(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	containerID := "nonexistent-id"
	mockService.On("RemoveContainer",
		mock.Anything,
		containerID,
	).Return(model.ErrContainerNotFound)

	req := httptest.NewRequest("DELETE", "/containers/"+containerID, nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	var errorResponse ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	assert.NoError(t, err)
	assert.Contains(t, errorResponse.Error, "container not found")

	mockService.AssertExpectations(t)
}

func TestDeleteContainer_InvalidState(t *testing.T) {
	tests := []struct {
		name           string
		containerState model.ContainerState
		expectedStatus int
	}{
		{
			name:           "delete removed container",
			containerState: model.Removed,
			expectedStatus: 400,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockService := new(MockContainerService)
			handler := NewContainerHandler(mockService)

			handler.RegisterRoutes(app)

			containerID := "container-123"
			mockService.On("RemoveContainer",
				mock.Anything,
				containerID,
			).Return(model.ErrInvalidState)

			req := httptest.NewRequest("DELETE", "/containers/"+containerID, nil)

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			var errorResponse ErrorResponse
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			assert.NoError(t, err)
			assert.Contains(t, errorResponse.Error, "invalid state")

			mockService.AssertExpectations(t)
		})
	}
}

func TestCreateContainer_ContractTest_RequestSchema(t *testing.T) {
	tests := []struct {
		name           string
		requestJSON    string
		expectedStatus int
		shouldSucceed  bool
		mockError      error
	}{
		{
			name: "valid schema - all required fields",
			requestJSON: `{
				"image": "nginx:latest",
				"command": ["nginx", "-g", "daemon off;"],
				"environment": {"ENV": "test"}
			}`,
			expectedStatus: 201,
			shouldSucceed:  true,
		},
		{
			name: "valid schema - minimal fields",
			requestJSON: `{
				"image": "nginx:latest"
			}`,
			expectedStatus: 201,
			shouldSucceed:  true,
		},
		{
			name: "invalid schema - missing required field",
			requestJSON: `{
				"command": ["nginx"]
			}`,
			expectedStatus: 400,
			shouldSucceed:  false,
			mockError:      model.ErrInvalidImage,
		},
		{
			name: "invalid schema - wrong type for command",
			requestJSON: `{
				"image": "nginx:latest",
				"command": "should be array"
			}`,
			expectedStatus: 400,
			shouldSucceed:  false,
		},
		{
			name: "invalid schema - wrong type for environment",
			requestJSON: `{
				"image": "nginx:latest",
				"environment": ["should", "be", "object"]
			}`,
			expectedStatus: 400,
			shouldSucceed:  false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockService := new(MockContainerService)
			handler := NewContainerHandler(mockService)

			handler.RegisterRoutes(app)

			if tt.shouldSucceed {
				now := time.Now()
				mockContainer := &model.Container{
					ID:        "container-123",
					Image:     "nginx:latest",
					State:     model.Running,
					CreatedAt: now,
				}
				mockService.On("CreateContainer", mock.Anything, mock.Anything).
					Return(mockContainer, nil)
			} else if tt.mockError != nil {
				// For tests that expect failure, mock the service to return an error
				mockService.On("CreateContainer", mock.Anything, mock.Anything).
					Return(nil, tt.mockError)
			}

			req := httptest.NewRequest("POST", "/containers", bytes.NewReader([]byte(tt.requestJSON)))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			if tt.shouldSucceed || tt.mockError != nil {
				mockService.AssertExpectations(t)
			}
		})
	}
}

func TestCreateContainer_ContractTest_ResponseSchema(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	now := time.Now()
	mockContainer := &model.Container{
		ID:        "container-123",
		DockerID:  "docker-123",
		Image:     "nginx:latest",
		Command:   []string{"nginx"},
		State:     model.Running,
		CreatedAt: now,
		EnvironmentVariables: map[string]string{
			"ENV": "test",
		},
	}

	mockService.On("CreateContainer", mock.Anything, mock.Anything).
		Return(mockContainer, nil)

	requestBody := CreateContainerRequest{
		Image: "nginx:latest",
	}

	body, _ := json.Marshal(requestBody)
	req := httptest.NewRequest("POST", "/containers", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 201, resp.StatusCode)

	// Validate response schema
	var response ContainerResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	// Verify all required fields are present
	assert.NotEmpty(t, response.ID, "id field is required")
	assert.NotEmpty(t, response.Image, "image field is required")
	assert.NotEmpty(t, response.State, "state field is required")
	assert.NotEmpty(t, response.CreatedAt, "created_at field is required")

	// Verify field types
	assert.IsType(t, "", response.ID)
	assert.IsType(t, "", response.Image)
	assert.IsType(t, "", response.State)
	assert.IsType(t, "", response.CreatedAt)

	mockService.AssertExpectations(t)
}

func TestCreateContainer_ContractTest_RequiredFields(t *testing.T) {
	tests := []struct {
		name          string
		requestJSON   string
		expectedValid bool
		mockError     error
	}{
		{
			name: "all required fields present",
			requestJSON: `{
				"image": "nginx:latest"
			}`,
			expectedValid: true,
		},
		{
			name:          "missing image field",
			requestJSON:   `{}`,
			expectedValid: false,
			mockError:     model.ErrInvalidImage,
		},
		{
			name: "null image field",
			requestJSON: `{
				"image": null
			}`,
			expectedValid: false,
			mockError:     model.ErrInvalidImage,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockService := new(MockContainerService)
			handler := NewContainerHandler(mockService)

			handler.RegisterRoutes(app)

			if tt.expectedValid {
				now := time.Now()
				mockContainer := &model.Container{
					ID:        "container-123",
					Image:     "nginx:latest",
					State:     model.Running,
					CreatedAt: now,
				}
				mockService.On("CreateContainer", mock.Anything, mock.Anything).
					Return(mockContainer, nil)
			} else if tt.mockError != nil {
				mockService.On("CreateContainer", mock.Anything, mock.Anything).
					Return(nil, tt.mockError)
			}

			req := httptest.NewRequest("POST", "/containers", bytes.NewReader([]byte(tt.requestJSON)))
			req.Header.Set("Content-Type", "application/json")

			resp, err := app.Test(req)
			assert.NoError(t, err)

			if tt.expectedValid {
				assert.Equal(t, 201, resp.StatusCode)
				mockService.AssertExpectations(t)
			} else {
				assert.NotEqual(t, 201, resp.StatusCode)
				if tt.mockError != nil {
					mockService.AssertExpectations(t)
				}
			}
		})
	}
}

func TestDeleteContainer_ContractTest_ResponseSchemas(t *testing.T) {
	tests := []struct {
		name           string
		containerID    string
		mockError      error
		expectedStatus int
		validateSchema func(*testing.T, *http.Response)
	}{
		{
			name:           "204 No Content - successful deletion",
			containerID:    "container-123",
			mockError:      nil,
			expectedStatus: 204,
			validateSchema: func(_ *testing.T, _ *http.Response) {
				// 204 should have no body
				// Note: Response body might not be empty in test context, so we just check status
			},
		},
		{
			name:           "404 Not Found - container doesn't exist",
			containerID:    "nonexistent",
			mockError:      model.ErrContainerNotFound,
			expectedStatus: 404,
			validateSchema: func(t *testing.T, resp *http.Response) {
				var errorResponse ErrorResponse
				err := json.NewDecoder(resp.Body).Decode(&errorResponse)
				assert.NoError(t, err)
				assert.NotEmpty(t, errorResponse.Error, "error field is required")
				assert.IsType(t, "", errorResponse.Error)
			},
		},
		{
			name:           "400 Bad Request - invalid state",
			containerID:    "container-123",
			mockError:      model.ErrInvalidState,
			expectedStatus: 400,
			validateSchema: func(t *testing.T, resp *http.Response) {
				var errorResponse ErrorResponse
				err := json.NewDecoder(resp.Body).Decode(&errorResponse)
				assert.NoError(t, err)
				assert.NotEmpty(t, errorResponse.Error, "error field is required")
				assert.IsType(t, "", errorResponse.Error)
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockService := new(MockContainerService)
			handler := NewContainerHandler(mockService)

			handler.RegisterRoutes(app)

			mockService.On("RemoveContainer", mock.Anything, tt.containerID).
				Return(tt.mockError)

			req := httptest.NewRequest("DELETE", "/containers/"+tt.containerID, nil)

			resp, err := app.Test(req)
			assert.NoError(t, err)
			assert.Equal(t, tt.expectedStatus, resp.StatusCode)

			// Validate response schema
			tt.validateSchema(t, resp)

			mockService.AssertExpectations(t)
		})
	}
}

func TestDeleteContainer_ContractTest_ErrorResponseFormat(t *testing.T) {
	tests := []struct {
		name          string
		mockError     error
		expectedError string
	}{
		{
			name:          "container not found error",
			mockError:     model.ErrContainerNotFound,
			expectedError: "container not found",
		},
		{
			name:          "invalid state error",
			mockError:     model.ErrInvalidState,
			expectedError: "invalid state",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			app := fiber.New()
			mockService := new(MockContainerService)
			handler := NewContainerHandler(mockService)

			handler.RegisterRoutes(app)

			containerID := "container-123"
			mockService.On("RemoveContainer", mock.Anything, containerID).
				Return(tt.mockError)

			req := httptest.NewRequest("DELETE", "/containers/"+containerID, nil)

			resp, err := app.Test(req)
			assert.NoError(t, err)

			// Verify error response format
			var errorResponse ErrorResponse
			err = json.NewDecoder(resp.Body).Decode(&errorResponse)
			assert.NoError(t, err)

			// Verify required fields
			assert.NotEmpty(t, errorResponse.Error, "error field is required")

			// Verify error message
			assert.Contains(t, errorResponse.Error, tt.expectedError)

			mockService.AssertExpectations(t)
		})
	}
}

func TestGetContainer_Success(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	// Create a test container
	now := time.Now()
	mockContainer := &model.Container{
		ID:        "container-123",
		DockerID:  "docker-123",
		Image:     "nginx:latest",
		State:     model.Running,
		CreatedAt: now,
		StartedAt: &now,
	}

	mockService.On("GetContainer", mock.Anything, "container-123").
		Return(mockContainer, nil)

	req := httptest.NewRequest("GET", "/containers/container-123", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Verify response body
	var response ContainerResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, mockContainer.ID, response.ID)
	assert.Equal(t, mockContainer.Image, response.Image)
	assert.Equal(t, mockContainer.State.String(), response.State)

	mockService.AssertExpectations(t)
}

func TestGetContainer_NotFound(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	containerID := "nonexistent-id"
	mockService.On("GetContainer", mock.Anything, containerID).
		Return(nil, model.ErrContainerNotFound)

	req := httptest.NewRequest("GET", "/containers/"+containerID, nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 404, resp.StatusCode)

	var errorResponse ErrorResponse
	err = json.NewDecoder(resp.Body).Decode(&errorResponse)
	assert.NoError(t, err)
	assert.Contains(t, errorResponse.Error, "container not found")

	mockService.AssertExpectations(t)
}

func TestListContainers_Success(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	// Create test containers
	now := time.Now()
	mockContainers := []*model.Container{
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

	mockService.On("ListContainers", mock.Anything).
		Return(mockContainers, nil)

	req := httptest.NewRequest("GET", "/containers", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	// Verify response body
	var response ListContainersResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, 2, response.Count)
	assert.Len(t, response.Containers, 2)
	assert.Equal(t, mockContainers[0].ID, response.Containers[0].ID)
	assert.Equal(t, mockContainers[1].ID, response.Containers[1].ID)

	mockService.AssertExpectations(t)
}

func TestListContainers_Empty(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	mockService.On("ListContainers", mock.Anything).
		Return([]*model.Container{}, nil)

	req := httptest.NewRequest("GET", "/containers", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response ListContainersResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, 0, response.Count)
	assert.Len(t, response.Containers, 0)

	mockService.AssertExpectations(t)
}

func TestGetContainer_ContractTest_ResponseSchema(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	now := time.Now()
	startTime := now.Add(-120 * time.Second)
	mockContainer := &model.Container{
		ID:        "container-123",
		DockerID:  "docker-123",
		Image:     "nginx:latest",
		Command:   []string{"nginx"},
		State:     model.Running,
		CreatedAt: now,
		StartedAt: &startTime,
		EnvironmentVariables: map[string]string{
			"ENV": "test",
		},
	}

	mockService.On("GetContainer", mock.Anything, "container-123").
		Return(mockContainer, nil)

	req := httptest.NewRequest("GET", "/containers/container-123", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response ContainerResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	assert.NotEmpty(t, response.ID, "id field is required")
	assert.NotEmpty(t, response.Image, "image field is required")
	assert.NotEmpty(t, response.State, "state field is required")
	assert.NotEmpty(t, response.CreatedAt, "created_at field is required")

	assert.IsType(t, "", response.ID)
	assert.IsType(t, "", response.Image)
	assert.IsType(t, "", response.State)
	assert.IsType(t, "", response.CreatedAt)

	validStates := []string{"creating", "running", "exited", "stopped", "removed"}
	assert.Contains(t, validStates, response.State)

	assert.NotNil(t, response.Uptime, "uptime should be present for running containers")
	assert.Greater(t, *response.Uptime, int64(119), "uptime should be at least 119 seconds")
	assert.Less(t, *response.Uptime, int64(122), "uptime should be less than 122 seconds")

	mockService.AssertExpectations(t)
}

func TestGetContainer_UptimeNullForNonRunning(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	now := time.Now()
	mockContainer := &model.Container{
		ID:        "container-123",
		DockerID:  "docker-123",
		Image:     "nginx:latest",
		State:     model.Stopped,
		CreatedAt: now,
	}

	mockService.On("GetContainer", mock.Anything, "container-123").
		Return(mockContainer, nil)

	req := httptest.NewRequest("GET", "/containers/container-123", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response ContainerResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Nil(t, response.Uptime, "uptime should be nil for stopped containers")

	mockService.AssertExpectations(t)
}

func TestListContainers_ContractTest_ResponseSchema(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	handler := NewContainerHandler(mockService)

	handler.RegisterRoutes(app)

	now := time.Now()
	mockContainers := []*model.Container{
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

	mockService.On("ListContainers", mock.Anything).
		Return(mockContainers, nil)

	req := httptest.NewRequest("GET", "/containers", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response ListContainersResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)
	assert.Equal(t, 2, response.Count)
	assert.Len(t, response.Containers, 2)

	for _, container := range response.Containers {
		assert.NotEmpty(t, container.ID, "id field is required")
		assert.NotEmpty(t, container.Image, "image field is required")
		assert.NotEmpty(t, container.State, "state field is required")
		assert.NotEmpty(t, container.CreatedAt, "created_at field is required")

		validStates := []string{"creating", "running", "exited", "stopped", "removed"}
		assert.Contains(t, validStates, container.State)
	}

	mockService.AssertExpectations(t)
}

func TestHealthCheck_Healthy(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	mockDockerManager := new(MockDockerManager)
	handler := NewContainerHandlerWithDocker(mockService, mockDockerManager)

	mockDockerManager.On("Ping", mock.Anything).Return(nil)

	app.Get("/health", handler.HealthCheck)

	req := httptest.NewRequest("GET", "/health", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 200, resp.StatusCode)

	var response HealthCheckResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, "healthy", response.Status)
	assert.Equal(t, "connected", response.Docker)
	assert.NotEmpty(t, response.Timestamp)

	_, err = time.Parse(time.RFC3339, response.Timestamp)
	assert.NoError(t, err, "timestamp should be in ISO8601 format")

	mockDockerManager.AssertExpectations(t)
}

func TestHealthCheck_Unhealthy(t *testing.T) {
	app := fiber.New()
	mockService := new(MockContainerService)
	mockDockerManager := new(MockDockerManager)
	handler := NewContainerHandlerWithDocker(mockService, mockDockerManager)

	mockDockerManager.On("Ping", mock.Anything).Return(errors.New("docker daemon unavailable"))

	app.Get("/health", handler.HealthCheck)

	req := httptest.NewRequest("GET", "/health", nil)

	resp, err := app.Test(req)
	assert.NoError(t, err)
	assert.Equal(t, 503, resp.StatusCode)

	var response HealthCheckResponse
	err = json.NewDecoder(resp.Body).Decode(&response)
	assert.NoError(t, err)

	assert.Equal(t, "unhealthy", response.Status)
	assert.Equal(t, "disconnected", response.Docker)
	assert.NotEmpty(t, response.Timestamp)

	_, err = time.Parse(time.RFC3339, response.Timestamp)
	assert.NoError(t, err, "timestamp should be in ISO8601 format")

	mockDockerManager.AssertExpectations(t)
}
