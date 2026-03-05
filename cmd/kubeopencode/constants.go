// Copyright Contributors to the KubeOpenCode project

package main

import (
	"os"
	"strconv"
)

// Common environment variable names shared across subcommands
const (
	envTaskName      = "TASK_NAME"
	envTaskNamespace = "TASK_NAMESPACE"
	envWorkspaceDir  = "WORKSPACE_DIR"
)

// Environment variable names for context-init
const (
	envConfigMapPath = "CONFIGMAP_PATH"
	envFileMappings  = "FILE_MAPPINGS"
	envDirMappings   = "DIR_MAPPINGS"
)

// getEnvOrDefault returns the value of the environment variable specified by key,
// or defaultValue if the environment variable is not set or empty.
func getEnvOrDefault(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvIntOrDefault returns the value of the environment variable specified by key
// parsed as an integer, or defaultValue if the environment variable is not set,
// empty, or not a valid integer.
func getEnvIntOrDefault(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
