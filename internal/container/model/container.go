package model

import (
	"fmt"
	"time"

	"github.com/google/uuid"
)

type ContainerState int

const (
	Creating ContainerState = iota
	Running
	Exited
	Stopped
	Removed
)

func (s ContainerState) String() string {
	switch s {
	case Creating:
		return "creating"
	case Running:
		return "running"
	case Exited:
		return "exited"
	case Stopped:
		return "stopped"
	case Removed:
		return "removed"
	default:
		return "unknown"
	}
}

func (s ContainerState) CanTransitionTo(target ContainerState) bool {
	validTransitions := map[ContainerState][]ContainerState{
		Creating: {Running, Exited},
		Running:  {Exited, Stopped},
		Exited:   {Removed},
		Stopped:  {Removed},
		Removed:  {},
	}

	allowed, exists := validTransitions[s]
	if !exists {
		return false
	}

	for _, state := range allowed {
		if state == target {
			return true
		}
	}
	return false
}

func (s ContainerState) IsTerminal() bool {
	return s == Exited || s == Removed
}

type ResourceLimits struct {
	MemoryMB int
	CPUCores float64
}

type RegistryCredentials struct {
	Username string
	Password string
}

type ContainerConfig struct {
	Image          string
	Command        []string
	Environment    map[string]string
	ResourceLimits *ResourceLimits
	RegistryAuth   *RegistryCredentials
}

type Container struct {
	ID                   string
	DockerID             string
	Image                string
	Command              []string
	EnvironmentVariables map[string]string
	State                ContainerState
	CreatedAt            time.Time
	StartedAt            *time.Time
	ExitedAt             *time.Time
	ExitCode             *int
	LastLogs             []string
	Labels               map[string]string
	ResourceLimits       *ResourceLimits
	ReconciledAt         *time.Time
}

func NewContainer(config *ContainerConfig) *Container {
	container := &Container{
		ID:                   uuid.New().String(),
		Image:                config.Image,
		Command:              config.Command,
		EnvironmentVariables: config.Environment,
		State:                Creating,
		CreatedAt:            time.Now(),
		ResourceLimits:       config.ResourceLimits,
	}
	container.Labels = CreateLabels(container)
	return container
}

func (c *Container) Start(dockerID string) error {
	if !c.State.CanTransitionTo(Running) {
		return fmt.Errorf("%w: cannot start container in %s state", ErrInvalidState, c.State.String())
	}

	c.State = Running
	c.DockerID = dockerID
	now := time.Now()
	c.StartedAt = &now
	return nil
}

func (c *Container) MarkExited(exitCode int, exitTime time.Time) error {
	if !c.State.CanTransitionTo(Exited) {
		return fmt.Errorf("%w: cannot mark container as exited in %s state", ErrInvalidState, c.State.String())
	}

	c.State = Exited
	c.ExitCode = &exitCode
	c.ExitedAt = &exitTime
	return nil
}

func (c *Container) Stop() error {
	if !c.State.CanTransitionTo(Stopped) {
		return fmt.Errorf("%w: cannot stop container in %s state", ErrInvalidState, c.State.String())
	}

	c.State = Stopped
	return nil
}

func (c *Container) Remove() error {
	if !c.State.CanTransitionTo(Removed) {
		return fmt.Errorf("%w: cannot remove container in %s state", ErrInvalidState, c.State.String())
	}

	c.State = Removed
	return nil
}

func (c *Container) IsRunning() bool {
	return c.State == Running
}

func (c *Container) Uptime() *int64 {
	if c.State != Running || c.StartedAt == nil {
		return nil
	}

	uptime := int64(time.Since(*c.StartedAt).Seconds())
	return &uptime
}

func (cfg *ContainerConfig) Validate() error {
	if err := validateImage(cfg.Image); err != nil {
		return err
	}

	if err := validateEnvironment(cfg.Environment); err != nil {
		return err
	}

	if cfg.ResourceLimits != nil {
		if err := cfg.ResourceLimits.Validate(); err != nil {
			return err
		}
	}

	return nil
}

func validateImage(image string) error {
	if image == "" {
		return ErrInvalidImage
	}

	for _, ch := range image {
		if !isValidImageChar(ch) {
			return ErrInvalidImage
		}
	}

	return nil
}

func isValidImageChar(ch rune) bool {
	return isAlphanumeric(ch) || isSpecialImageChar(ch)
}

func isAlphanumeric(ch rune) bool {
	return (ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')
}

func isSpecialImageChar(ch rune) bool {
	return ch == '.' || ch == '-' || ch == '_' || ch == ':' || ch == '/' || ch == '@'
}

func validateEnvironment(env map[string]string) error {
	if env == nil {
		return nil
	}

	const maxEnvSize = 64 * 1024
	totalSize := 0

	for key, value := range env {
		totalSize += len(key) + len(value)
		if totalSize > maxEnvSize {
			return ErrEnvironmentTooLarge
		}
	}

	return nil
}

func (rl *ResourceLimits) Validate() error {
	if rl == nil {
		return nil
	}

	if rl.MemoryMB > 0 && (rl.MemoryMB < 4 || rl.MemoryMB > 32768) {
		return ErrResourceLimitExceeded
	}

	if rl.CPUCores > 0 && (rl.CPUCores < 0.01 || rl.CPUCores > 64.0) {
		return ErrResourceLimitExceeded
	}

	return nil
}
