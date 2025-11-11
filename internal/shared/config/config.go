package config

import (
	"os"
	"strconv"
	"time"
)

type Config struct {
	Port             string
	MaxContainers    int
	DockerHost       string
	ContainerTimeout int
	ShutdownTimeout  int
	ImagePullTimeout time.Duration
}

func LoadConfig() *Config {
	return &Config{
		Port:             getEnv("PORT", "3000"),
		MaxContainers:    getEnvInt("MAX_CONTAINERS", 50),
		DockerHost:       getEnv("DOCKER_HOST", "unix:///var/run/docker.sock"),
		ContainerTimeout: getEnvInt("CONTAINER_TIMEOUT", 300),
		ShutdownTimeout:  getEnvInt("SHUTDOWN_TIMEOUT", 30),
		ImagePullTimeout: getEnvDuration("IMAGE_PULL_TIMEOUT", 5*time.Minute),
	}
}

func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}

func getEnvDuration(key string, defaultValue time.Duration) time.Duration {
	if value := os.Getenv(key); value != "" {
		if duration, err := time.ParseDuration(value); err == nil {
			if duration > 15*time.Minute {
				return 15 * time.Minute
			}
			return duration
		}
	}
	return defaultValue
}
