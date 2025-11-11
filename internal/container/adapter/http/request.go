package http

import (
	"github.com/cyucelen/ignite/internal/container/model"
)

type RegistryAuthRequest struct {
	Username string `json:"username" validate:"omitempty"`
	Password string `json:"password" validate:"omitempty"`
}

type CreateContainerRequest struct {
	Image        string               `json:"image" validate:"required"`
	Command      []string             `json:"command,omitempty"`
	Environment  map[string]string    `json:"environment,omitempty"`
	RegistryAuth *RegistryAuthRequest `json:"registryAuth,omitempty"`
}

func (r *CreateContainerRequest) ToContainerConfig() *model.ContainerConfig {
	config := &model.ContainerConfig{
		Image:       r.Image,
		Command:     r.Command,
		Environment: r.Environment,
	}

	if r.RegistryAuth != nil {
		config.RegistryAuth = &model.RegistryCredentials{
			Username: r.RegistryAuth.Username,
			Password: r.RegistryAuth.Password,
		}
	}

	return config
}
