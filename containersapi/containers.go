package containersapi

import (
	"context"
	"log"
	"tlex/tlextypes"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

const containerRunningStateString string = "running"

// GetContainersLogReaders gets our running containers' log readers
func GetContainersLogReaders(cli *client.Client, ownedContainersID tlextypes.OwnedContainersID) []tlextypes.ContainerReaderStream {

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Panicf("Unable to list containers. Error: %v", err)
	}

	containerLogStreams := []tlextypes.ContainerReaderStream{}

	for _, container := range containers {
		hostPort := ownedContainersID[container.ID]
		if hostPort > 0 && container.State == containerRunningStateString {
			readerStream, err := cli.ContainerLogs(context.Background(), container.ID, types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     true,
			})
			if err != nil {
				log.Panicf("Unable to solicit a log reader from the container %s, error: %s\n", container.ID, err)
			}

			containerLogStream := tlextypes.ContainerReaderStream{readerStream, hostPort}
			containerLogStreams = append(containerLogStreams, containerLogStream)
		}
	}

	return containerLogStreams
}

// GetContainersStatsReaders gets our running containers' resources readers
func GetContainersStatsReaders(cli *client.Client, ownedContainersID tlextypes.OwnedContainersID) []tlextypes.ContainerReaderStream {

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Panicf("Unable to list containers. Error: %v", err)
	}

	containerStatsStreams := []tlextypes.ContainerReaderStream{}

	for _, container := range containers {
		hostPort := ownedContainersID[container.ID]
		if hostPort > 0 && container.State == containerRunningStateString {
			out, err := cli.ContainerStats(context.Background(), container.ID, true)
			if err != nil {
				log.Panicf("Unable to solicit a monitoring reader from the container %s, error: %s\n", container.ID, err)
			}

			containerStatsStream := tlextypes.ContainerReaderStream{out.Body, hostPort}
			containerStatsStreams = append(containerStatsStreams, containerStatsStream)
		}
	}

	return containerStatsStreams
}
