package storage

import (
	"context"
	"sync"
	"testing"
	"time"

	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInMemoryRepository_Save(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	container := &model.Container{
		ID:                   uuid.New().String(),
		DockerID:             "docker123",
		Image:                "alpine:latest",
		State:                model.Running,
		CreatedAt:            time.Now(),
		EnvironmentVariables: map[string]string{"KEY": "value"},
	}

	err := repo.Save(ctx, container)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, container.ID)
	require.NoError(t, err)
	assert.Equal(t, container.ID, found.ID)
	assert.Equal(t, container.DockerID, found.DockerID)
	assert.Equal(t, container.Image, found.Image)
	assert.Equal(t, container.State, found.State)
}

func TestInMemoryRepository_Save_Update(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	container := &model.Container{
		ID:        uuid.New().String(),
		DockerID:  "docker123",
		Image:     "alpine:latest",
		State:     model.Creating,
		CreatedAt: time.Now(),
	}

	err := repo.Save(ctx, container)
	require.NoError(t, err)

	container.State = model.Running
	startedAt := time.Now()
	container.StartedAt = &startedAt

	err = repo.Save(ctx, container)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, container.ID)
	require.NoError(t, err)
	assert.Equal(t, model.Running, found.State)
	assert.NotNil(t, found.StartedAt)
}

func TestInMemoryRepository_FindByID_Exists(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	container := &model.Container{
		ID:        uuid.New().String(),
		DockerID:  "docker123",
		Image:     "alpine:latest",
		State:     model.Running,
		CreatedAt: time.Now(),
	}

	err := repo.Save(ctx, container)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, container.ID)
	require.NoError(t, err)
	assert.Equal(t, container.ID, found.ID)
	assert.Equal(t, container.DockerID, found.DockerID)
}

func TestInMemoryRepository_FindByID_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	nonExistentID := uuid.New().String()

	found, err := repo.FindByID(ctx, nonExistentID)
	assert.Nil(t, found)
	assert.ErrorIs(t, err, model.ErrContainerNotFound)
}

func TestInMemoryRepository_FindAll_Empty(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	containers, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Empty(t, containers)
	assert.NotNil(t, containers)
}

func TestInMemoryRepository_FindAll_Multiple(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	container1 := &model.Container{
		ID:        uuid.New().String(),
		DockerID:  "docker1",
		Image:     "alpine:latest",
		State:     model.Running,
		CreatedAt: time.Now(),
	}
	container2 := &model.Container{
		ID:        uuid.New().String(),
		DockerID:  "docker2",
		Image:     "nginx:latest",
		State:     model.Exited,
		CreatedAt: time.Now(),
	}
	container3 := &model.Container{
		ID:        uuid.New().String(),
		DockerID:  "docker3",
		Image:     "redis:latest",
		State:     model.Running,
		CreatedAt: time.Now(),
	}

	require.NoError(t, repo.Save(ctx, container1))
	require.NoError(t, repo.Save(ctx, container2))
	require.NoError(t, repo.Save(ctx, container3))

	containers, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Len(t, containers, 3)

	ids := make(map[string]bool)
	for _, c := range containers {
		ids[c.ID] = true
	}
	assert.True(t, ids[container1.ID])
	assert.True(t, ids[container2.ID])
	assert.True(t, ids[container3.ID])
}

func TestInMemoryRepository_Delete_Exists(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	container := &model.Container{
		ID:        uuid.New().String(),
		DockerID:  "docker123",
		Image:     "alpine:latest",
		State:     model.Running,
		CreatedAt: time.Now(),
	}

	err := repo.Save(ctx, container)
	require.NoError(t, err)

	err = repo.Delete(ctx, container.ID)
	require.NoError(t, err)

	found, err := repo.FindByID(ctx, container.ID)
	assert.Nil(t, found)
	assert.ErrorIs(t, err, model.ErrContainerNotFound)
}

func TestInMemoryRepository_Delete_NotFound(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	nonExistentID := uuid.New().String()

	err := repo.Delete(ctx, nonExistentID)
	assert.ErrorIs(t, err, model.ErrContainerNotFound)
}

func TestInMemoryRepository_CountActive_MixedStates(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	containers := []*model.Container{
		{
			ID:        uuid.New().String(),
			DockerID:  "docker1",
			Image:     "alpine:latest",
			State:     model.Creating,
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			DockerID:  "docker2",
			Image:     "nginx:latest",
			State:     model.Running,
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			DockerID:  "docker3",
			Image:     "redis:latest",
			State:     model.Exited,
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			DockerID:  "docker4",
			Image:     "postgres:latest",
			State:     model.Running,
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			DockerID:  "docker5",
			Image:     "mongo:latest",
			State:     model.Removed,
			CreatedAt: time.Now(),
		},
		{
			ID:        uuid.New().String(),
			DockerID:  "docker6",
			Image:     "mysql:latest",
			State:     model.Stopped,
			CreatedAt: time.Now(),
		},
	}

	for _, c := range containers {
		require.NoError(t, repo.Save(ctx, c))
	}

	count, err := repo.CountActive(ctx)
	require.NoError(t, err)
	assert.Equal(t, 4, count)
}

func TestInMemoryRepository_CountActive_Empty(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	count, err := repo.CountActive(ctx)
	require.NoError(t, err)
	assert.Equal(t, 0, count)
}

func TestInMemoryRepository_ConcurrentAccess(t *testing.T) {
	ctx := context.Background()
	repo := NewInMemoryRepository()

	const numGoroutines = 10
	const numOpsPerGoroutine = 100

	var wg sync.WaitGroup
	wg.Add(numGoroutines * 3)

	for i := 0; i < numGoroutines; i++ {
		go func(index int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				container := &model.Container{
					ID:        uuid.New().String(),
					DockerID:  uuid.New().String(),
					Image:     "alpine:latest",
					State:     model.Running,
					CreatedAt: time.Now(),
				}
				_ = repo.Save(ctx, container)
			}
		}(i)

		go func(index int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				_, _ = repo.FindAll(ctx)
			}
		}(i)

		go func(index int) {
			defer wg.Done()
			for j := 0; j < numOpsPerGoroutine; j++ {
				_, _ = repo.CountActive(ctx)
			}
		}(i)
	}

	wg.Wait()

	containers, err := repo.FindAll(ctx)
	require.NoError(t, err)
	assert.Equal(t, numGoroutines*numOpsPerGoroutine, len(containers))
}
