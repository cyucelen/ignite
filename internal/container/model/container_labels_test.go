package model

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// T006: Unit tests for label constants
func TestLabelConstants(t *testing.T) {
	// Verify label constants are defined correctly
	assert.Equal(t, "ignite.managed", LabelManaged)
	assert.Equal(t, "ignite.id", LabelID)
	assert.Equal(t, "ignite.created-at", LabelCreatedAt)
	assert.Equal(t, "ignite.image", LabelImage)
	assert.Equal(t, "ignite.version", LabelVersion)
}

// T007: Unit tests for CreateLabels() - valid container input
func TestCreateLabels_ValidContainer(t *testing.T) {
	// Create test container
	config := &ContainerConfig{
		Image:   "alpine:latest",
		Command: []string{"echo", "test"},
	}
	container := NewContainer(config)
	container.CreatedAt = time.Date(2025, 11, 10, 10, 30, 0, 0, time.UTC)

	// Call CreateLabels
	labels := CreateLabels(container)

	// Verify all 5 labels present
	assert.Len(t, labels, 5)
	assert.Equal(t, "true", labels[LabelManaged])
	assert.Equal(t, container.ID, labels[LabelID])
	assert.Equal(t, "2025-11-10T10:30:00Z", labels[LabelCreatedAt])
	assert.Equal(t, "alpine:latest", labels[LabelImage])
	assert.NotEmpty(t, labels[LabelVersion]) // Version should be set
}

// T008: Unit tests for ParseLabels() - valid labels
func TestParseLabels_ValidLabels(t *testing.T) {
	validLabels := map[string]string{
		LabelManaged:   "true",
		LabelID:        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		LabelCreatedAt: "2025-11-10T10:30:00Z",
		LabelImage:     "alpine:latest",
		LabelVersion:   "1.0.0",
	}

	containerLabels, err := ParseLabels(validLabels)

	require.NoError(t, err)
	assert.True(t, containerLabels.Managed)
	assert.Equal(t, "f47ac10b-58cc-4372-a567-0e02b2c3d479", containerLabels.ID)
	assert.Equal(t, time.Date(2025, 11, 10, 10, 30, 0, 0, time.UTC), containerLabels.CreatedAt)
	assert.Equal(t, "alpine:latest", containerLabels.Image)
	assert.Equal(t, "1.0.0", containerLabels.Version)
}

// T009: Unit tests for ParseLabels() - missing ignite.managed label
func TestParseLabels_MissingManagedLabel(t *testing.T) {
	labelsWithoutManaged := map[string]string{
		LabelID:        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		LabelCreatedAt: "2025-11-10T10:30:00Z",
		LabelImage:     "alpine:latest",
		LabelVersion:   "1.0.0",
	}

	_, err := ParseLabels(labelsWithoutManaged)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "not an ignite container")
}

// T010: Unit tests for ParseLabels() - invalid UUID in ignite.id
func TestParseLabels_InvalidUUID(t *testing.T) {
	labelsWithInvalidUUID := map[string]string{
		LabelManaged:   "true",
		LabelID:        "not-a-valid-uuid",
		LabelCreatedAt: "2025-11-10T10:30:00Z",
		LabelImage:     "alpine:latest",
		LabelVersion:   "1.0.0",
	}

	_, err := ParseLabels(labelsWithInvalidUUID)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ignite.id")
}

// T011: Unit tests for ParseLabels() - invalid RFC3339 timestamp
func TestParseLabels_InvalidTimestamp(t *testing.T) {
	labelsWithInvalidTimestamp := map[string]string{
		LabelManaged:   "true",
		LabelID:        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
		LabelCreatedAt: "not-a-valid-timestamp",
		LabelImage:     "alpine:latest",
		LabelVersion:   "1.0.0",
	}

	_, err := ParseLabels(labelsWithInvalidTimestamp)

	assert.Error(t, err)
	assert.Contains(t, err.Error(), "invalid ignite.created-at")
}

// T012: Unit tests for ParseLabels() - missing required labels
func TestParseLabels_MissingRequiredLabels(t *testing.T) {
	tests := []struct {
		name          string
		labels        map[string]string
		expectedError string
	}{
		{
			name: "missing ignite.id",
			labels: map[string]string{
				LabelManaged:   "true",
				LabelCreatedAt: "2025-11-10T10:30:00Z",
				LabelImage:     "alpine:latest",
				LabelVersion:   "1.0.0",
			},
			expectedError: "invalid ignite.id",
		},
		{
			name: "missing ignite.image",
			labels: map[string]string{
				LabelManaged:   "true",
				LabelID:        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
				LabelCreatedAt: "2025-11-10T10:30:00Z",
				LabelVersion:   "1.0.0",
			},
			expectedError: "missing ignite.image",
		},
		{
			name: "missing ignite.version",
			labels: map[string]string{
				LabelManaged:   "true",
				LabelID:        "f47ac10b-58cc-4372-a567-0e02b2c3d479",
				LabelCreatedAt: "2025-11-10T10:30:00Z",
				LabelImage:     "alpine:latest",
			},
			expectedError: "missing ignite.version",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := ParseLabels(tt.labels)
			assert.Error(t, err)
			assert.Contains(t, err.Error(), tt.expectedError)
		})
	}
}

// T007: Additional test - CreateLabels with different image names
func TestCreateLabels_DifferentImages(t *testing.T) {
	images := []string{
		"nginx:latest",
		"postgres:14",
		"redis:alpine",
		"my-custom-image:v1.2.3",
	}

	for _, image := range images {
		config := &ContainerConfig{Image: image}
		container := NewContainer(config)

		labels := CreateLabels(container)

		assert.Equal(t, image, labels[LabelImage])
		// Verify UUID is valid
		_, err := uuid.Parse(labels[LabelID])
		assert.NoError(t, err)
	}
}
