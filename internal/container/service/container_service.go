package service

import (
	"context"
	"fmt"
	"log"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/cyucelen/ignite/internal/shared/config"
)

type ContainerEvent struct {
	DockerID  string
	EventType string
	ExitCode  *int
	Timestamp time.Time
}

type DockerManager interface {
	CreateContainer(ctx context.Context, container *model.Container, creds *model.RegistryCredentials) (string, error)
	StartContainer(ctx context.Context, dockerID string) error
	StopContainer(ctx context.Context, dockerID string) error
	RemoveContainer(ctx context.Context, dockerID string) error
	InspectContainer(ctx context.Context, dockerID string) (*model.Container, error)
	WatchEvents(ctx context.Context) (<-chan ContainerEvent, <-chan error)
	GetLogs(ctx context.Context, dockerID string, tail int, timestamps bool) ([]string, error)
	ListIgniteContainers(ctx context.Context) ([]types.ContainerJSON, error)
}

type ContainerRepository interface {
	Save(ctx context.Context, container *model.Container) error
	FindByID(ctx context.Context, id string) (*model.Container, error)
	FindAll(ctx context.Context) ([]*model.Container, error)
	Delete(ctx context.Context, id string) error
	CountActive(ctx context.Context) (int, error)
}

type ContainerService struct {
	dockerManager DockerManager
	repository    ContainerRepository
	config        *config.Config
	semaphore     chan struct{}
}

func NewContainerService(dockerManager DockerManager, repository ContainerRepository, cfg *config.Config) *ContainerService {
	return &ContainerService{
		dockerManager: dockerManager,
		repository:    repository,
		config:        cfg,
		semaphore:     make(chan struct{}, cfg.MaxContainers),
	}
}

func (s *ContainerService) CreateContainer(ctx context.Context, config *model.ContainerConfig) (*model.Container, error) {
	if err := config.Validate(); err != nil {
		return nil, err
	}

	count, err := s.repository.CountActive(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to count active containers: %w", err)
	}

	if count >= s.config.MaxContainers {
		return nil, model.ErrContainerLimitReached
	}

	container := model.NewContainer(config)

	dockerID, err := s.dockerManager.CreateContainer(ctx, container, config.RegistryAuth)
	if err != nil {
		return nil, fmt.Errorf("failed to create container in docker: %w", err)
	}

	container.DockerID = dockerID

	if err := s.repository.Save(ctx, container); err != nil {
		return nil, fmt.Errorf("failed to save container: %w", err)
	}

	if err := s.dockerManager.StartContainer(ctx, dockerID); err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	now := time.Now()
	container.State = model.Running
	container.StartedAt = &now

	if err := s.repository.Save(ctx, container); err != nil {
		return nil, fmt.Errorf("failed to update container state: %w", err)
	}

	return container, nil
}

func (s *ContainerService) StartContainer(ctx context.Context, id string) error {
	container, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.dockerManager.StartContainer(ctx, container.DockerID); err != nil {
		return fmt.Errorf("failed to start container: %w", err)
	}

	now := time.Now()
	container.State = model.Running
	container.StartedAt = &now

	return s.repository.Save(ctx, container)
}

func (s *ContainerService) StopContainer(ctx context.Context, id string) error {
	container, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if err := s.dockerManager.StopContainer(ctx, container.DockerID); err != nil {
		return fmt.Errorf("failed to stop container: %w", err)
	}

	if err := container.Stop(); err != nil {
		return err
	}

	return s.repository.Save(ctx, container)
}

func (s *ContainerService) RemoveContainer(ctx context.Context, id string) error {
	container, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return err
	}

	if container.State == model.Running {
		if err := s.dockerManager.StopContainer(ctx, container.DockerID); err != nil {
			return fmt.Errorf("failed to stop container before removal: %w", err)
		}
		if err := container.Stop(); err != nil {
			return err
		}
		if err := s.repository.Save(ctx, container); err != nil {
			return fmt.Errorf("failed to save container state: %w", err)
		}
	}

	if err := s.dockerManager.RemoveContainer(ctx, container.DockerID); err != nil {
		return fmt.Errorf("failed to remove container from docker: %w", err)
	}

	if err := container.Remove(); err != nil {
		return err
	}

	return s.repository.Save(ctx, container)
}

func (s *ContainerService) GetContainer(ctx context.Context, id string) (*model.Container, error) {
	return s.repository.FindByID(ctx, id)
}

func (s *ContainerService) ListContainers(ctx context.Context) ([]*model.Container, error) {
	return s.repository.FindAll(ctx)
}

func (s *ContainerService) CleanupExitedContainers(ctx context.Context) error {
	containers, err := s.repository.FindAll(ctx)
	if err != nil {
		return fmt.Errorf("failed to list containers: %w", err)
	}

	timeout := time.Duration(s.config.ContainerTimeout) * time.Second
	now := time.Now()

	for _, container := range containers {
		if container.State == model.Exited && container.ExitedAt != nil {
			if now.Sub(*container.ExitedAt) > timeout {
				if err := s.dockerManager.RemoveContainer(ctx, container.DockerID); err != nil {
					continue
				}

				if err := container.Remove(); err != nil {
					continue
				}

				if err := s.repository.Save(ctx, container); err != nil {
					continue
				}
			}
		}
	}

	return nil
}

func (s *ContainerService) GetLogs(ctx context.Context, id string, tail int, timestamps bool) ([]string, error) {
	container, err := s.repository.FindByID(ctx, id)
	if err != nil {
		return nil, err
	}

	logs, err := s.dockerManager.GetLogs(ctx, container.DockerID, tail, timestamps)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs: %w", err)
	}

	return logs, nil
}

func (s *ContainerService) ReconcileContainers(ctx context.Context) (int, error) {
	log.Printf("[reconciliation.start] Beginning startup reconciliation")

	dockerContainers, err := s.dockerManager.ListIgniteContainers(ctx)
	if err != nil {
		return 0, fmt.Errorf("failed to list containers: %w", err)
	}

	log.Printf("[reconciliation.found] Found %d existing Ignite containers", len(dockerContainers))

	reconciledCount := 0
	for _, dockerJSON := range dockerContainers {
		container, err := ReconstructContainer(dockerJSON)
		if err != nil {
			log.Printf("[reconciliation.skip] Skipping container %s: %v", dockerJSON.ID, err)
			continue
		}

		if err := s.repository.Save(ctx, container); err != nil {
			log.Printf("[reconciliation.save_failed] Failed to save container %s: %v", container.ID, err)
			continue
		}

		reconciledCount++
		dockerIDShort := container.DockerID
		if len(dockerIDShort) > 12 {
			dockerIDShort = dockerIDShort[:12]
		}
		log.Printf("[reconciliation.success] Reconciled container %s (Docker: %s, State: %s)",
			container.ID, dockerIDShort, container.State.String())
	}

	log.Printf("[reconciliation.complete] Reconciled %d/%d containers successfully",
		reconciledCount, len(dockerContainers))

	return reconciledCount, nil
}

func (s *ContainerService) HandleContainerEvents(ctx context.Context) {
	eventChan, errChan := s.dockerManager.WatchEvents(ctx)

	for {
		select {
		case <-ctx.Done():
			return
		case err := <-errChan:
			if err != nil {
				continue
			}
		case event := <-eventChan:
			s.processEvent(ctx, event)
		}
	}
}

func (s *ContainerService) processEvent(ctx context.Context, event ContainerEvent) {
	container := s.findContainerByDockerID(ctx, event.DockerID)
	if container == nil {
		return
	}

	s.handleEventByType(ctx, container, event)
}

func (s *ContainerService) findContainerByDockerID(ctx context.Context, dockerID string) *model.Container {
	containers, err := s.repository.FindAll(ctx)
	if err != nil {
		return nil
	}

	for _, c := range containers {
		if c.DockerID == dockerID {
			return c
		}
	}
	return nil
}

func (s *ContainerService) handleEventByType(ctx context.Context, container *model.Container, event ContainerEvent) {
	switch event.EventType {
	case "die":
		s.handleDieEvent(ctx, container, event)
	case "start":
		s.handleStartEvent(ctx, container, event)
	case "stop":
		s.handleStopEvent(ctx, container)
	}
}

func (s *ContainerService) handleDieEvent(ctx context.Context, container *model.Container, event ContainerEvent) {
	if event.ExitCode == nil {
		return
	}
	if err := container.MarkExited(*event.ExitCode, event.Timestamp); err != nil {
		return
	}
	_ = s.repository.Save(ctx, container)
}

func (s *ContainerService) handleStartEvent(ctx context.Context, container *model.Container, event ContainerEvent) {
	container.State = model.Running
	container.StartedAt = &event.Timestamp
	_ = s.repository.Save(ctx, container)
}

func (s *ContainerService) handleStopEvent(ctx context.Context, container *model.Container) {
	if err := container.Stop(); err != nil {
		return
	}
	_ = s.repository.Save(ctx, container)
}
