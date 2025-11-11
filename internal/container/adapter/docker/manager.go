package docker

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	networktypes "github.com/docker/docker/api/types/network"
	"github.com/docker/docker/api/types/registry"
	"github.com/docker/docker/errdefs"
	"github.com/cyucelen/ignite/internal/container/model"
	"github.com/cyucelen/ignite/internal/container/service"
	"github.com/cyucelen/ignite/internal/shared/config"
	v1 "github.com/opencontainers/image-spec/specs-go/v1"
)

//nolint:revive
type DockerClient interface {
	ContainerCreate(ctx context.Context, config *container.Config, hostConfig *container.HostConfig, networkingConfig *networktypes.NetworkingConfig, platformConfig *v1.Platform, containerName string) (container.CreateResponse, error)
	ContainerStart(ctx context.Context, containerID string, options container.StartOptions) error
	ContainerStop(ctx context.Context, containerID string, options container.StopOptions) error
	ContainerRemove(ctx context.Context, containerID string, options container.RemoveOptions) error
	ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error)
	ContainerList(ctx context.Context, options container.ListOptions) ([]types.Container, error)
	ImageList(ctx context.Context, options types.ImageListOptions) ([]image.Summary, error)
	ImagePull(ctx context.Context, refStr string, options types.ImagePullOptions) (io.ReadCloser, error)
	Events(ctx context.Context, options types.EventsOptions) (<-chan events.Message, <-chan error)
	ContainerLogs(ctx context.Context, containerID string, options container.LogsOptions) (io.ReadCloser, error)
	Close() error
	Ping(ctx context.Context) (types.Ping, error)
}

type pullOperation struct {
	done chan struct{}
	err  error
	mu   sync.Mutex
}

//nolint:revive
type DockerManager struct {
	client      DockerClient
	config      *config.Config
	activePulls sync.Map
}

func NewDockerManager(client DockerClient, cfg *config.Config) *DockerManager {
	return &DockerManager{
		client:      client,
		config:      cfg,
		activePulls: sync.Map{},
	}
}

func (m *DockerManager) CreateContainer(ctx context.Context, cont *model.Container, creds *model.RegistryCredentials) (string, error) {
	if err := m.EnsureImage(ctx, cont.Image, creds); err != nil {
		return "", fmt.Errorf("failed to ensure image availability: %w", err)
	}

	config := &container.Config{
		Image:  cont.Image,
		Cmd:    cont.Command,
		Labels: cont.Labels,
	}

	if len(cont.EnvironmentVariables) > 0 {
		env := make([]string, 0, len(cont.EnvironmentVariables))
		for key, value := range cont.EnvironmentVariables {
			env = append(env, fmt.Sprintf("%s=%s", key, value))
		}
		config.Env = env
	}

	hostConfig := &container.HostConfig{}

	if cont.ResourceLimits != nil {
		if cont.ResourceLimits.MemoryMB > 0 {
			hostConfig.Memory = int64(cont.ResourceLimits.MemoryMB) * 1024 * 1024
		}

		if cont.ResourceLimits.CPUCores > 0 {
			hostConfig.CPUPeriod = 100000
			hostConfig.CPUQuota = int64(cont.ResourceLimits.CPUCores * float64(hostConfig.CPUPeriod))
		}
	}

	resp, err := m.client.ContainerCreate(
		ctx,
		config,
		hostConfig,
		nil,
		nil,
		"",
	)

	if err != nil {
		return "", fmt.Errorf("failed to create container: %w", err)
	}

	return resp.ID, nil
}

func (m *DockerManager) StartContainer(ctx context.Context, dockerID string) error {
	if err := m.client.ContainerStart(ctx, dockerID, container.StartOptions{}); err != nil {
		return fmt.Errorf("failed to start container %s: %w", dockerID, err)
	}
	return nil
}

func (m *DockerManager) StopContainer(ctx context.Context, dockerID string) error {
	timeout := 10
	stopOptions := container.StopOptions{
		Timeout: &timeout,
	}

	if err := m.client.ContainerStop(ctx, dockerID, stopOptions); err != nil {
		return fmt.Errorf("failed to stop container %s: %w", dockerID, err)
	}
	return nil
}

func (m *DockerManager) RemoveContainer(ctx context.Context, dockerID string) error {
	removeOptions := container.RemoveOptions{
		Force:         true,
		RemoveVolumes: true,
	}

	if err := m.client.ContainerRemove(ctx, dockerID, removeOptions); err != nil {
		if errdefs.IsNotFound(err) {
			return nil
		}
		return fmt.Errorf("failed to remove container %s: %w", dockerID, err)
	}
	return nil
}

func (m *DockerManager) InspectContainer(ctx context.Context, dockerID string) (*model.Container, error) {
	containerJSON, err := m.client.ContainerInspect(ctx, dockerID)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container %s: %w", dockerID, err)
	}

	container := &model.Container{
		DockerID:             containerJSON.ID,
		Image:                containerJSON.Config.Image,
		Command:              containerJSON.Config.Cmd,
		State:                mapDockerState(containerJSON.State),
		StartedAt:            parseTimestamp(containerJSON.State.StartedAt),
		ExitedAt:             parseTimestamp(containerJSON.State.FinishedAt),
		ExitCode:             getExitCode(containerJSON.State),
		Labels:               containerJSON.Config.Labels,
		EnvironmentVariables: parseEnvironmentVariables(containerJSON.Config.Env),
	}

	return container, nil
}

func mapDockerState(state *types.ContainerState) model.ContainerState {
	if state.Running {
		return model.Running
	}
	if state.Dead || state.OOMKilled {
		return model.Exited
	}
	if state.Restarting {
		return model.Creating
	}
	return model.Stopped
}

func parseTimestamp(timestamp string) *time.Time {
	if timestamp == "" {
		return nil
	}
	parsed, err := time.Parse(time.RFC3339Nano, timestamp)
	if err != nil || parsed.IsZero() {
		return nil
	}
	return &parsed
}

func getExitCode(state *types.ContainerState) *int {
	if state.ExitCode != 0 || !state.Running {
		code := state.ExitCode
		return &code
	}
	return nil
}

func parseEnvironmentVariables(env []string) map[string]string {
	if len(env) == 0 {
		return nil
	}

	result := make(map[string]string)
	for _, e := range env {
		key, value := splitEnvVar(e)
		if key != "" {
			result[key] = value
		}
	}
	return result
}

func splitEnvVar(env string) (key, value string) {
	for i, ch := range env {
		if ch == '=' {
			return env[:i], env[i+1:]
		}
	}
	return "", ""
}

func extractRegistryHost(imageName string) string {
	if !strings.Contains(imageName, "/") {
		return ""
	}

	parts := strings.Split(imageName, "/")
	if len(parts) > 0 {
		first := parts[0]
		if strings.Contains(first, ".") || strings.Contains(first, ":") {
			return first
		}
	}
	return ""
}

func redactPassword(msg string, password string) string {
	if password == "" {
		return msg
	}
	return strings.ReplaceAll(msg, password, "[REDACTED]")
}

func (m *DockerManager) encodeAuthCredentials(imageName string, creds *model.RegistryCredentials) (string, error) {
	if creds == nil {
		return "", nil
	}

	registryHost := extractRegistryHost(imageName)

	authConfig := registry.AuthConfig{
		Username:      creds.Username,
		Password:      creds.Password,
		ServerAddress: registryHost,
	}

	encoded, err := registry.EncodeAuthConfig(authConfig)
	if err != nil {
		return "", fmt.Errorf("failed to encode auth config: %w", err)
	}

	return encoded, nil
}

func (m *DockerManager) WatchEvents(ctx context.Context) (<-chan service.ContainerEvent, <-chan error) {
	eventChan := make(chan service.ContainerEvent, 100)
	errChan := make(chan error, 1)

	filter := filters.NewArgs()
	filter.Add("type", "container")
	filter.Add("label", "ignite.managed=true")

	dockerEventChan, dockerErrChan := m.client.Events(ctx, types.EventsOptions{
		Filters: filter,
	})

	go func() {
		defer close(eventChan)
		defer close(errChan)

		for {
			select {
			case <-ctx.Done():
				return
			case err := <-dockerErrChan:
				if err != nil {
					errChan <- err
				}
				return
			case msg := <-dockerEventChan:
				event := m.convertDockerEvent(msg)
				if event != nil {
					eventChan <- *event
				}
			}
		}
	}()

	return eventChan, errChan
}

func (m *DockerManager) convertDockerEvent(msg events.Message) *service.ContainerEvent {
	if msg.Type != "container" {
		return nil
	}

	event := &service.ContainerEvent{
		DockerID:  msg.Actor.ID,
		EventType: string(msg.Action),
		Timestamp: time.Unix(msg.Time, 0),
	}

	if string(msg.Action) == "die" {
		if exitCodeStr, ok := msg.Actor.Attributes["exitCode"]; ok {
			if exitCode, err := strconv.Atoi(exitCodeStr); err == nil {
				event.ExitCode = &exitCode
			}
		}
	}

	return event
}

func (m *DockerManager) Ping(ctx context.Context) error {
	_, err := m.client.Ping(ctx)
	return err
}

func (m *DockerManager) GetLogs(ctx context.Context, dockerID string, tail int, timestamps bool) ([]string, error) {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
		Timestamps: timestamps,
		Tail:       fmt.Sprintf("%d", tail),
	}

	reader, err := m.client.ContainerLogs(ctx, dockerID, options)
	if err != nil {
		return nil, fmt.Errorf("failed to get logs for container %s: %w", dockerID, err)
	}
	defer func() { _ = reader.Close() }()

	logs, err := io.ReadAll(reader)
	if err != nil {
		return nil, fmt.Errorf("failed to read logs: %w", err)
	}

	lines := m.parseDockerLogs(logs)
	return lines, nil
}

func (m *DockerManager) parseDockerLogs(data []byte) []string {
	var lines []string
	pos := 0

	for pos < len(data) {
		if pos+8 > len(data) {
			break
		}

		size := int(data[pos+4])<<24 | int(data[pos+5])<<16 | int(data[pos+6])<<8 | int(data[pos+7])
		pos += 8

		if pos+size > len(data) {
			break
		}

		line := string(data[pos : pos+size])
		lines = append(lines, line)
		pos += size
	}

	return lines
}

func (m *DockerManager) Close() error {
	return m.client.Close()
}

func (m *DockerManager) ListIgniteContainers(ctx context.Context) ([]types.ContainerJSON, error) {
	log.Printf("[docker.list.start] Listing Ignite-managed containers")

	filter := filters.NewArgs()
	filter.Add("label", "ignite.managed=true")

	containers, err := m.client.ContainerList(ctx, container.ListOptions{
		All:     true,
		Filters: filter,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to list containers: %w", err)
	}

	log.Printf("[docker.list.found] Found %d Ignite-managed containers", len(containers))

	result := make([]types.ContainerJSON, 0, len(containers))
	for _, cont := range containers {
		inspected, err := m.client.ContainerInspect(ctx, cont.ID)
		if err != nil {
			log.Printf("[docker.list.skip] Failed to inspect container %s: %v", cont.ID[:12], err)
			continue
		}
		result = append(result, inspected)
	}

	log.Printf("[docker.list.complete] Successfully inspected %d/%d containers",
		len(result), len(containers))

	return result, nil
}

func (m *DockerManager) EnsureImage(ctx context.Context, imageName string, creds *model.RegistryCredentials) error {
	log.Printf("[image.check.start] Checking image availability: %s", imageName)

	exists, err := m.checkImageExists(ctx, imageName)
	if err != nil {
		return fmt.Errorf("failed to check image existence: %w", err)
	}

	if exists {
		log.Printf("[image.check.cached] Image found in local cache: %s", imageName)
		return nil
	}

	log.Printf("[image.pull.start] Image not cached, pulling: %s", imageName)

	newOp := &pullOperation{done: make(chan struct{})}
	actual, loaded := m.activePulls.LoadOrStore(imageName, newOp)
	op := actual.(*pullOperation)

	if loaded {
		log.Printf("[image.pull.wait] Waiting for concurrent pull to complete: %s", imageName)
		select {
		case <-op.done:
			op.mu.Lock()
			err := op.err
			op.mu.Unlock()
			return err
		case <-ctx.Done():
			return ctx.Err()
		}
	}

	defer func() {
		m.activePulls.Delete(imageName)
		close(op.done)
	}()

	pullErr := m.pullImage(ctx, imageName, creds)
	op.mu.Lock()
	op.err = pullErr
	op.mu.Unlock()

	return pullErr
}

func (m *DockerManager) checkImageExists(ctx context.Context, imageName string) (bool, error) {
	filter := filters.NewArgs()
	filter.Add("reference", imageName)

	images, err := m.client.ImageList(ctx, types.ImageListOptions{
		Filters: filter,
	})
	if err != nil {
		return false, err
	}

	return len(images) > 0, nil
}

func (m *DockerManager) pullImage(ctx context.Context, imageName string, creds *model.RegistryCredentials) error {
	pullCtx, cancel := context.WithTimeout(ctx, m.config.ImagePullTimeout)
	defer cancel()

	startTime := time.Now()

	authStr, err := m.encodeAuthCredentials(imageName, creds)
	if err != nil {
		return fmt.Errorf("failed to encode credentials: %w", err)
	}

	out, err := m.client.ImagePull(pullCtx, imageName, types.ImagePullOptions{
		RegistryAuth: authStr,
	})
	if err != nil {
		duration := time.Since(startTime)
		classifiedErr := model.ClassifyPullError(err, imageName)
		log.Printf("[image.pull.failed] Image pull failed: %s (duration: %dms, error: %v)",
			imageName, duration.Milliseconds(), classifiedErr)
		return classifiedErr
	}
	defer func() { _ = out.Close() }()

	if err := m.parsePullProgress(out, imageName); err != nil {
		duration := time.Since(startTime)
		log.Printf("[image.pull.failed] Failed to parse pull progress: %s (duration: %dms, error: %v)",
			imageName, duration.Milliseconds(), err)
		return model.ClassifyPullError(err, imageName)
	}

	duration := time.Since(startTime)
	log.Printf("[image.pull.complete] Image pull completed: %s (duration: %dms)",
		imageName, duration.Milliseconds())

	return nil
}

func (m *DockerManager) parsePullProgress(reader io.Reader, imageName string) error {
	decoder := json.NewDecoder(reader)

	for {
		var progress struct {
			Status string `json:"status"`
			ID     string `json:"id"`
			Error  string `json:"error"`
			Detail struct {
				Current int64 `json:"current"`
				Total   int64 `json:"total"`
			} `json:"progressDetail"`
		}

		if err := decoder.Decode(&progress); err != nil {
			if err == io.EOF {
				return nil
			}
			return err
		}

		if progress.Error != "" {
			return fmt.Errorf("pull error: %s", progress.Error)
		}

		if progress.Status == "Downloading" && progress.Detail.Total > 0 {
			percentage := float64(progress.Detail.Current) / float64(progress.Detail.Total) * 100
			if percentage > 0 && int(percentage)%25 == 0 {
				log.Printf("[image.pull.progress] Pulling %s: layer %s - %.0f%%",
					imageName, progress.ID, percentage)
			}
		}
	}
}
