package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/docker/docker/client"
	"github.com/cyucelen/ignite/internal/container/adapter/docker"
	httpAdapter "github.com/cyucelen/ignite/internal/container/adapter/http"
	"github.com/cyucelen/ignite/internal/container/adapter/storage"
	"github.com/cyucelen/ignite/internal/container/service"
	"github.com/cyucelen/ignite/internal/shared/config"
	"github.com/cyucelen/ignite/internal/ui"
	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/logger"
	"github.com/gofiber/fiber/v2/middleware/recover"
	"github.com/google/uuid"
)

func main() {
	cfg := config.LoadConfig()
	logStartup(cfg)

	dockerClient, err := initializeDockerClient()
	if err != nil {
		log.Fatalf("Failed to initialize Docker: %v", err)
	}
	defer func() { _ = dockerClient.Close() }()

	dockerManager := docker.NewDockerManager(dockerClient, cfg)
	repository := storage.NewInMemoryRepository()
	containerService := service.NewContainerService(dockerManager, repository, cfg)

	reconcileExistingContainers(containerService)

	broadcaster := httpAdapter.NewBroadcaster()
	go broadcaster.Run()
	log.Println("Started WebSocket broadcaster")

	eventCtx, eventCancel := context.WithCancel(context.Background())
	defer eventCancel()
	go containerService.HandleContainerEvents(eventCtx)
	log.Println("Started container event handler")

	app := initializeHTTPServer(containerService, dockerManager, broadcaster)
	serverErr := startServer(app, cfg.Port)
	gracefulShutdown(app, eventCancel, cfg, serverErr)
}

func logStartup(cfg *config.Config) {
	log.Printf("Starting Ignite API server on port %s", cfg.Port)
	log.Printf("Max containers: %d", cfg.MaxContainers)
	log.Printf("Container timeout: %d seconds", cfg.ContainerTimeout)
	log.Printf("Image pull timeout: %v", cfg.ImagePullTimeout)
}

func initializeDockerClient() (*client.Client, error) {
	dockerClient, err := client.NewClientWithOpts(
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create Docker client: %w", err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if _, err := dockerClient.Ping(ctx); err != nil {
		_ = dockerClient.Close()
		return nil, fmt.Errorf("failed to connect to Docker daemon: %w", err)
	}
	log.Println("Successfully connected to Docker daemon")

	return dockerClient, nil
}

func initializeHTTPServer(containerService *service.ContainerService, dockerManager *docker.DockerManager, broadcaster *httpAdapter.Broadcaster) *fiber.App {
	app := fiber.New(fiber.Config{
		DisableStartupMessage: false,
		AppName:               "Ignite API",
		ErrorHandler:          customErrorHandler,
	})

	app.Use(recover.New())
	app.Use(logger.New(logger.Config{
		Format: "[${time}] ${status} - ${latency} ${method} ${path}\n",
	}))

	handler := httpAdapter.NewContainerHandlerWithDocker(containerService, dockerManager)

	api := app.Group("/api")
	api.Post("/containers", handler.CreateContainer)
	api.Post("/containers/:id/stop", handler.StopContainer)
	api.Get("/containers", handler.ListContainers)
	api.Get("/containers/:id/logs", handler.GetLogs)
	api.Get("/containers/:id", handler.GetContainer)
	api.Delete("/containers/:id", handler.DeleteContainer)
	api.Get("/health", handler.HealthCheck)

	app.Get("/ws/containers", websocket.New(func(c *websocket.Conn) {
		client := &httpAdapter.Client{
			ID:   uuid.New().String(),
			Conn: c,
			Send: make(chan []byte, 256),
		}

		broadcaster.Register(client)

		go httpAdapter.WritePump(client, broadcaster)
		httpAdapter.ReadPump(client, broadcaster)
	}))

	if err := ui.SetupRoutes(app); err != nil {
		log.Fatalf("Failed to setup UI routes: %v", err)
	}

	return app
}

func startServer(app *fiber.App, port string) chan error {
	serverErr := make(chan error, 1)
	go func() {
		if err := app.Listen(":" + port); err != nil {
			serverErr <- err
		}
	}()
	log.Printf("Server started on port %s", port)
	return serverErr
}

func gracefulShutdown(app *fiber.App, eventCancel context.CancelFunc, cfg *config.Config, serverErr chan error) {
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	select {
	case err := <-serverErr:
		log.Fatalf("Server error: %v", err)
	case sig := <-quit:
		log.Printf("Received signal: %v", sig)
	}

	log.Println("Shutting down server...")
	eventCancel()
	shutdownServer(app, cfg)
	log.Println("Server shutdown complete")
}

func shutdownServer(app *fiber.App, cfg *config.Config) {
	shutdownCtx, shutdownCancel := context.WithTimeout(
		context.Background(),
		time.Duration(cfg.ShutdownTimeout)*time.Second,
	)
	defer shutdownCancel()

	if err := app.ShutdownWithContext(shutdownCtx); err != nil {
		log.Printf("Error during server shutdown: %v", err)
	}
}

func reconcileExistingContainers(containerService *service.ContainerService) {
	log.Println("Reconciling existing containers...")
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	count, err := containerService.ReconcileContainers(ctx)
	if err != nil {
		log.Printf("Warning: Reconciliation failed: %v", err)
		return
	}

	if count > 0 {
		log.Printf("Successfully reconciled %d existing container(s)", count)
	} else {
		log.Println("No existing containers found")
	}
}

func customErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError

	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}

	return c.Status(code).JSON(fiber.Map{
		"error": err.Error(),
	})
}
