package model

import (
	"errors"
	"fmt"
	"strings"
)

var (
	ErrInvalidImage          = errors.New("invalid container image format")
	ErrInvalidEnvironment    = errors.New("invalid environment variables")
	ErrInvalidState          = errors.New("container is in invalid state for this operation")
	ErrContainerNotFound     = errors.New("container not found")
	ErrResourceLimitExceeded = errors.New("resource limits exceed allowed values")
	ErrEnvironmentTooLarge   = errors.New("environment variables exceed size limit")
	ErrContainerLimitReached = errors.New("container limit reached")
	ErrInvalidContainerID    = errors.New("invalid container ID")

	ErrImageNotFound        = errors.New("image not found in registry")
	ErrRegistryUnreachable  = errors.New("container registry is unavailable")
	ErrPullTimeout          = errors.New("image pull timeout")
	ErrAuthRequired         = errors.New("image requires authentication")
	ErrRateLimited          = errors.New("registry rate limit exceeded")
	ErrInsufficientDisk     = errors.New("insufficient disk space for image")

	ErrInvalidMetrics    = errors.New("invalid metrics values")
	ErrInvalidLogStream  = errors.New("invalid log stream type")
	ErrInvalidLogEntry   = errors.New("invalid log entry")
	ErrInvalidLogFilter  = errors.New("invalid log filter criteria")
	ErrInvalidTimeRange  = errors.New("invalid time range: since must be before until")
)

func ClassifyPullError(err error, imageName string) error {
	if err == nil {
		return nil
	}

	errMsg := strings.ToLower(err.Error())

	switch {
	case strings.Contains(errMsg, "unauthorized"), strings.Contains(errMsg, "authentication required"):
		return fmt.Errorf("%w for image: %s", ErrAuthRequired, imageName)
	case strings.Contains(errMsg, "denied: requested access to the resource is denied"):
		return fmt.Errorf("%w for image: %s", ErrAuthRequired, imageName)
	case strings.Contains(errMsg, "not found"), strings.Contains(errMsg, "does not exist"):
		return fmt.Errorf("%w: %s", ErrImageNotFound, imageName)
	case strings.Contains(errMsg, "no such host"), strings.Contains(errMsg, "dial tcp"):
		return fmt.Errorf("%w: %v", ErrRegistryUnreachable, err)
	case strings.Contains(errMsg, "context deadline exceeded"), strings.Contains(errMsg, "timeout"):
		return fmt.Errorf("%w: %v", ErrPullTimeout, err)
	case strings.Contains(errMsg, "too many requests"), strings.Contains(errMsg, "rate limit"):
		return fmt.Errorf("%w: %v", ErrRateLimited, err)
	case strings.Contains(errMsg, "no space left"), strings.Contains(errMsg, "insufficient space"):
		return fmt.Errorf("%w: %v", ErrInsufficientDisk, err)
	default:
		return fmt.Errorf("image pull failed for %s: %w", imageName, err)
	}
}
