package storage

import (
	"context"
	"sync"

	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/cyucelen/ignite/internal/container/service"
)

type InMemoryRepository struct {
	mu         sync.RWMutex
	containers map[string]*model.Container
}

func NewInMemoryRepository() *InMemoryRepository {
	return &InMemoryRepository{
		containers: make(map[string]*model.Container),
	}
}

var _ service.ContainerRepository = (*InMemoryRepository)(nil)

func (r *InMemoryRepository) Save(_ context.Context, container *model.Container) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.containers[container.ID] = container
	return nil
}

func (r *InMemoryRepository) FindByID(_ context.Context, id string) (*model.Container, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	container, exists := r.containers[id]
	if !exists {
		return nil, model.ErrContainerNotFound
	}
	return container, nil
}

func (r *InMemoryRepository) FindAll(_ context.Context) ([]*model.Container, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	containers := make([]*model.Container, 0, len(r.containers))
	for _, c := range r.containers {
		containers = append(containers, c)
	}
	return containers, nil
}

func (r *InMemoryRepository) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	if _, exists := r.containers[id]; !exists {
		return model.ErrContainerNotFound
	}
	delete(r.containers, id)
	return nil
}

func (r *InMemoryRepository) CountActive(_ context.Context) (int, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()

	count := 0
	for _, c := range r.containers {
		if c.State != model.Exited && c.State != model.Removed {
			count++
		}
	}
	return count, nil
}
