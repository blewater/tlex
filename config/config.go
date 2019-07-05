// Package config holds defaults + desired application parameters for its operation.
package config

import (
	"os"
	"tlex/helper"
)

// AppConfig holds the app configuration values
type AppConfig struct {
	DockerFilename               string
	DockerImageName              string
	DockerExposedPort            int
	RequestedLiveContainers      int
	StartingHTTPServerNattedPort int
	ContainerRunningStateString  string
	LogFilename                  string
	StatsFilename                string
	StatsPersist                 bool
	StatsDisplay                 bool
	// display every modulus throttleStatsInputRequests
	ThrottleStatsInputRequests int
	// Used for unit testing to wait on channels to sync up with unit tests
	InTestingModeWithChannelsSync bool
}

// GetConfig returns the application parameters.
func GetConfig() AppConfig {

	config := AppConfig{

		DockerFilename:               helper.GetCWD() + string(os.PathSeparator) + "Dockerfile",
		DockerImageName:              "mariohellowebserver:latest",
		DockerExposedPort:            8770,
		RequestedLiveContainers:      2,
		StartingHTTPServerNattedPort: 8770,
		ContainerRunningStateString:  "running",
		LogFilename:                  helper.GetCWD() + string(os.PathSeparator) + "containers.log",
		StatsFilename:                helper.GetCWD() + string(os.PathSeparator) + "containers_stats.log",
		StatsPersist:                 true,
		StatsDisplay:                 true,
		ThrottleStatsInputRequests:   20,

		//*** Note if InTestingModeWithChannelsSync is set to true during
		// normal operation it will wait on the containersChecked channel after erasing the containers.
		// Used only for unit tests requiring being notified for intermediate state changes.

		InTestingModeWithChannelsSync: false,
	}

	return config
}
