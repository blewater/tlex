// Package config holds defaults + desired application parameters for its operation.
package config

// AppConfig holds the app configuration values
type AppConfig struct {

	DockerFilename string
	DockerImageName string
	DockerExposedPort int
	RequestedLiveContainers int
	StartingHTTPServerNattedPort int
	ContainerRunningStateString string
	LogFilename string
	StatsFilename string
	StatsPersist bool
	StatsDisplay bool
	// display every modulus throttleStatsInputRequests
	ThrottleStatsInputRequests int

}

// GetConfig returns the application parameters.
func GetConfig() AppConfig {

	config := AppConfig {

		DockerFilename : "Dockerfile",
		DockerImageName : "mariohellowebserver:latest",
		DockerExposedPort : 8770,
		RequestedLiveContainers : 2,
		StartingHTTPServerNattedPort : 8770,
		ContainerRunningStateString : "running",
		LogFilename : "containers.log",
		StatsFilename : "containers_stats.log",
		StatsPersist : true,
		StatsDisplay : true,
		ThrottleStatsInputRequests : 20,
	}

	return config
}