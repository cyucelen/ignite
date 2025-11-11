package http

import (
	"context"
	"errors"
	"log"
	"time"

	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/gofiber/fiber/v2"
)

// ContainerService defines the interface for container service operations
type ContainerService interface {
	CreateContainer(ctx context.Context, config *model.ContainerConfig) (*model.Container, error)
	GetContainer(ctx context.Context, id string) (*model.Container, error)
	ListContainers(ctx context.Context) ([]*model.Container, error)
	RemoveContainer(ctx context.Context, id string) error
	StartContainer(ctx context.Context, id string) error
	StopContainer(ctx context.Context, id string) error
	GetLogs(ctx context.Context, id string, tail int, timestamps bool) ([]string, error)
}

type DockerManager interface {
	Ping(ctx context.Context) error
}

type ContainerHandler struct {
	service       ContainerService
	dockerManager DockerManager
}

func NewContainerHandler(service ContainerService) *ContainerHandler {
	return &ContainerHandler{
		service:       service,
		dockerManager: nil,
	}
}

func NewContainerHandlerWithDocker(service ContainerService, dockerManager DockerManager) *ContainerHandler {
	return &ContainerHandler{
		service:       service,
		dockerManager: dockerManager,
	}
}

func (h *ContainerHandler) HealthCheck(c *fiber.Ctx) error {
	timestamp := time.Now().Format(time.RFC3339)

	if h.dockerManager == nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(&HealthCheckResponse{
			Status:    "unhealthy",
			Docker:    "disconnected",
			Timestamp: timestamp,
		})
	}

	if err := h.dockerManager.Ping(c.Context()); err != nil {
		return c.Status(fiber.StatusServiceUnavailable).JSON(&HealthCheckResponse{
			Status:    "unhealthy",
			Docker:    "disconnected",
			Timestamp: timestamp,
		})
	}

	return c.Status(fiber.StatusOK).JSON(&HealthCheckResponse{
		Status:    "healthy",
		Docker:    "connected",
		Timestamp: timestamp,
	})
}

func (h *ContainerHandler) RegisterRoutes(app *fiber.App) {
	app.Post("/containers", h.CreateContainer)
	app.Post("/containers/:id/stop", h.StopContainer)
	app.Delete("/containers/:id", h.DeleteContainer)
	app.Get("/containers/:id", h.GetContainer)
	app.Get("/containers/:id/logs", h.GetLogs)
	app.Get("/containers", h.ListContainers)
}

func (h *ContainerHandler) CreateContainer(c *fiber.Ctx) error {
	var req CreateContainerRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(
			errors.New("invalid request body"),
		))
	}

	config := req.ToContainerConfig()

	container, err := h.service.CreateContainer(c.Context(), config)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(NewContainerResponse(container))
}

func (h *ContainerHandler) DeleteContainer(c *fiber.Ctx) error {
	id := c.Params("id")

	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(
			errors.New("container id is required"),
		))
	}

	if err := h.service.RemoveContainer(c.Context(), id); err != nil {
		return h.handleError(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (h *ContainerHandler) StopContainer(c *fiber.Ctx) error {
	id := c.Params("id")

	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(
			errors.New("container id is required"),
		))
	}

	if err := h.service.StopContainer(c.Context(), id); err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"message": "Container stopped successfully",
	})
}

func (h *ContainerHandler) GetContainer(c *fiber.Ctx) error {
	id := c.Params("id")

	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(
			errors.New("container id is required"),
		))
	}

	container, err := h.service.GetContainer(c.Context(), id)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(NewContainerResponse(container))
}

func (h *ContainerHandler) ListContainers(c *fiber.Ctx) error {
	containers, err := h.service.ListContainers(c.Context())
	if err != nil {
		return h.handleError(c, err)
	}

	responses := make([]*ContainerResponse, len(containers))
	for i, container := range containers {
		responses[i] = NewContainerResponse(container)
	}

	return c.JSON(&ListContainersResponse{
		Containers: responses,
		Count:      len(responses),
	})
}

func (h *ContainerHandler) GetLogs(c *fiber.Ctx) error {
	id := c.Params("id")

	if id == "" {
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(
			errors.New("container id is required"),
		))
	}

	tail := c.QueryInt("tail", 100)
	timestamps := c.QueryBool("timestamps", true)

	logs, err := h.service.GetLogs(c.Context(), id, tail, timestamps)
	if err != nil {
		return h.handleError(c, err)
	}

	return c.JSON(fiber.Map{
		"logs": logs,
	})
}

//nolint:cyclop
func (h *ContainerHandler) handleError(c *fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, model.ErrContainerNotFound):
		log.Printf("Container not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrImageNotFound):
		log.Printf("Image not found: %v", err)
		return c.Status(fiber.StatusNotFound).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrInvalidImage):
		log.Printf("Invalid image: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrAuthRequired):
		log.Printf("Authentication required: %v", err)
		return c.Status(fiber.StatusUnauthorized).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrRateLimited):
		log.Printf("Rate limited: %v", err)
		return c.Status(fiber.StatusTooManyRequests).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrRegistryUnreachable):
		log.Printf("Registry unreachable: %v", err)
		return c.Status(fiber.StatusServiceUnavailable).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrPullTimeout):
		log.Printf("Image pull timeout: %v", err)
		return c.Status(fiber.StatusGatewayTimeout).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrInsufficientDisk):
		log.Printf("Insufficient disk space: %v", err)
		return c.Status(fiber.StatusInsufficientStorage).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrInvalidEnvironment):
		log.Printf("Invalid environment: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrEnvironmentTooLarge):
		log.Printf("Environment too large: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrInvalidState):
		log.Printf("Invalid state: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrResourceLimitExceeded):
		log.Printf("Resource limit exceeded: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(NewErrorResponse(err))
	case errors.Is(err, model.ErrContainerLimitReached):
		log.Printf("Container limit reached: %v", err)
		return c.Status(fiber.StatusTooManyRequests).JSON(NewErrorResponse(err))
	default:
		log.Printf("Internal server error: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(NewErrorResponse(
			errors.New("internal server error"),
		))
	}
}
