package model

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestDomainErrors_ErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrInvalidImage",
			err:      ErrInvalidImage,
			expected: "invalid container image format",
		},
		{
			name:     "ErrInvalidEnvironment",
			err:      ErrInvalidEnvironment,
			expected: "invalid environment variables",
		},
		{
			name:     "ErrInvalidState",
			err:      ErrInvalidState,
			expected: "container is in invalid state for this operation",
		},
		{
			name:     "ErrContainerNotFound",
			err:      ErrContainerNotFound,
			expected: "container not found",
		},
		{
			name:     "ErrResourceLimitExceeded",
			err:      ErrResourceLimitExceeded,
			expected: "resource limits exceed allowed values",
		},
		{
			name:     "ErrEnvironmentTooLarge",
			err:      ErrEnvironmentTooLarge,
			expected: "environment variables exceed size limit",
		},
		{
			name:     "ErrContainerLimitReached",
			err:      ErrContainerLimitReached,
			expected: "container limit reached",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}

func TestDomainErrors_IsCheck(t *testing.T) {
	// Test that errors.Is works correctly
	assert.True(t, errors.Is(ErrInvalidImage, ErrInvalidImage))
	assert.False(t, errors.Is(ErrInvalidImage, ErrContainerNotFound))
}

func TestClassifyPullError_NilError(t *testing.T) {
	err := ClassifyPullError(nil, "nginx:alpine")
	assert.Nil(t, err)
}

func TestClassifyPullError_ImageNotFound(t *testing.T) {
	tests := []struct {
		name      string
		inputErr  error
		imageName string
	}{
		{
			name:      "manifest not found",
			inputErr:  errors.New("manifest for nginx:invalid-tag not found"),
			imageName: "nginx:invalid-tag",
		},
		{
			name:      "repository does not exist",
			inputErr:  errors.New("Error response from daemon: repository nonexistent/image does not exist"),
			imageName: "nonexistent/image:latest",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifyPullError(tt.inputErr, tt.imageName)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrImageNotFound))
			assert.Contains(t, err.Error(), tt.imageName)
		})
	}
}

func TestClassifyPullError_RegistryUnreachable(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
	}{
		{
			name:     "no such host",
			inputErr: errors.New("Get https://invalid-registry.io/v2/: dial tcp: lookup invalid-registry.io: no such host"),
		},
		{
			name:     "connection refused",
			inputErr: errors.New("dial tcp 127.0.0.1:5000: connect: connection refused"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifyPullError(tt.inputErr, "nginx:alpine")
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrRegistryUnreachable))
		})
	}
}

func TestClassifyPullError_PullTimeout(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
	}{
		{
			name:     "context deadline exceeded",
			inputErr: errors.New("context deadline exceeded"),
		},
		{
			name:     "timeout",
			inputErr: errors.New("Get https://registry-1.docker.io/v2/: net/http: request canceled (Client.Timeout exceeded while awaiting headers)"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifyPullError(tt.inputErr, "tensorflow/tensorflow:latest")
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrPullTimeout))
		})
	}
}

func TestClassifyPullError_AuthRequired(t *testing.T) {
	tests := []struct {
		name      string
		inputErr  error
		imageName string
	}{
		{
			name:      "unauthorized",
			inputErr:  errors.New("Error response from daemon: pull access denied for myregistry.io/private/app, repository does not exist or may require 'docker login': denied: requested access to the resource is denied"),
			imageName: "myregistry.io/private/app:latest",
		},
		{
			name:      "authentication required",
			inputErr:  errors.New("unauthorized: authentication required"),
			imageName: "private-repo/app:v1.0.0",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifyPullError(tt.inputErr, tt.imageName)
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrAuthRequired))
			assert.Contains(t, err.Error(), tt.imageName)
		})
	}
}

func TestClassifyPullError_RateLimited(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
	}{
		{
			name:     "too many requests",
			inputErr: errors.New("Error response from daemon: toomanyrequests: You have reached your pull rate limit. You may increase the limit by authenticating and upgrading: https://www.docker.com/increase-rate-limit"),
		},
		{
			name:     "rate limit exceeded",
			inputErr: errors.New("rate limit exceeded: 100 pulls per 6 hours"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifyPullError(tt.inputErr, "nginx:alpine")
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrRateLimited))
		})
	}
}

func TestClassifyPullError_InsufficientDisk(t *testing.T) {
	tests := []struct {
		name     string
		inputErr error
	}{
		{
			name:     "no space left on device",
			inputErr: errors.New("Error response from daemon: write /var/lib/docker/tmp/docker-import-123456789: no space left on device"),
		},
		{
			name:     "insufficient space",
			inputErr: errors.New("insufficient space available on the device"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ClassifyPullError(tt.inputErr, "postgres:15")
			assert.Error(t, err)
			assert.True(t, errors.Is(err, ErrInsufficientDisk))
		})
	}
}

func TestClassifyPullError_UnknownError(t *testing.T) {
	inputErr := errors.New("some unexpected error from Docker daemon")
	err := ClassifyPullError(inputErr, "nginx:alpine")

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "image pull failed")
	assert.Contains(t, err.Error(), "nginx:alpine")
	assert.Contains(t, err.Error(), "some unexpected error")
}

func TestImagePullErrors_ErrorMessages(t *testing.T) {
	tests := []struct {
		name     string
		err      error
		expected string
	}{
		{
			name:     "ErrImageNotFound",
			err:      ErrImageNotFound,
			expected: "image not found in registry",
		},
		{
			name:     "ErrRegistryUnreachable",
			err:      ErrRegistryUnreachable,
			expected: "container registry is unavailable",
		},
		{
			name:     "ErrPullTimeout",
			err:      ErrPullTimeout,
			expected: "image pull timeout",
		},
		{
			name:     "ErrAuthRequired",
			err:      ErrAuthRequired,
			expected: "image requires authentication",
		},
		{
			name:     "ErrRateLimited",
			err:      ErrRateLimited,
			expected: "registry rate limit exceeded",
		},
		{
			name:     "ErrInsufficientDisk",
			err:      ErrInsufficientDisk,
			expected: "insufficient disk space for image",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			assert.Equal(t, tt.expected, tt.err.Error())
		})
	}
}
