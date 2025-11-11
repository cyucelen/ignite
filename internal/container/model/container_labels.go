package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

const (
	LabelManaged   = "ignite.managed"
	LabelID        = "ignite.id"
	LabelCreatedAt = "ignite.created-at"
	LabelImage     = "ignite.image"
	LabelVersion   = "ignite.version"
)

var IgniteVersion = "1.0.0"

type ContainerLabels struct {
	Managed   bool
	ID        string
	CreatedAt time.Time
	Image     string
	Version   string
}

func CreateLabels(container *Container) map[string]string {
	return map[string]string{
		LabelManaged:   "true",
		LabelID:        container.ID,
		LabelCreatedAt: container.CreatedAt.Format(time.RFC3339),
		LabelImage:     container.Image,
		LabelVersion:   IgniteVersion,
	}
}

func ParseLabels(dockerLabels map[string]string) (*ContainerLabels, error) {
	if dockerLabels[LabelManaged] != "true" {
		return nil, fmt.Errorf("not an ignite container: ignite.managed != true")
	}

	id := dockerLabels[LabelID]
	if id == "" {
		return nil, fmt.Errorf("invalid ignite.id: empty")
	}
	if _, err := uuid.Parse(id); err != nil {
		return nil, fmt.Errorf("invalid ignite.id: %w", err)
	}

	createdAtStr := dockerLabels[LabelCreatedAt]
	if createdAtStr == "" {
		return nil, fmt.Errorf("invalid ignite.created-at: empty")
	}
	createdAt, err := time.Parse(time.RFC3339, createdAtStr)
	if err != nil {
		return nil, fmt.Errorf("invalid ignite.created-at: %w", err)
	}

	image := dockerLabels[LabelImage]
	if image == "" {
		return nil, fmt.Errorf("missing ignite.image")
	}

	version := dockerLabels[LabelVersion]
	if version == "" {
		return nil, fmt.Errorf("missing ignite.version")
	}

	return &ContainerLabels{
		Managed:   true,
		ID:        id,
		CreatedAt: createdAt,
		Image:     image,
		Version:   version,
	}, nil
}
