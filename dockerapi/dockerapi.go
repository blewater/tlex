// Package dockerapi is the facade to the Docker remote api.
package dockerapi

import (
	"context"
	"fmt"
	"io"
	"log"
	"os"

	"tlex/helper"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/go-connections/nat"
)

const containerRunningStateString string = "running"

// OwnedContainers contains the containers ID created by this process
type OwnedContainers map[string]int

// ContainerReaderStream contains a container reader stream, host port mapped to the container http server.
type ContainerReaderStream struct {
	ReaderStream io.ReadCloser
	HostPort     int
}

// GetDockerClient returns a docker remote api client handle value foundational to all Docker remote api interactions.
// Upon error it panics.
// Note for this tool the following stateful tactic:
// This process creates one docker client when launching and holds on to it for all API interactions.
// This should be contrasted with the stateless approach of requesting a new client for any API interaction.
func GetDockerClient() *client.Client {

	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Panicf("Docker client.NewClientWithOpts error: %s\n", err)
	}
	dockerClient.NegotiateAPIVersion(ctx)

	return dockerClient
}

// BuildDockerImage builds a Docker Image for a given dockerFilename located in the same folder
// of the running process
func BuildDockerImage(dockerClient *client.Client, dockerFilename string) {

	dockerFilePath := helper.GetCWD() + string(os.PathSeparator) + dockerFilename
	tarDockerfileReader, err := archive.TarWithOptions(dockerFilePath, &archive.TarOptions{})
	if err != nil {
		log.Fatal(err, " :unable to create tar with Dockerfile")
	}
	log.Printf("Building Docker Image in %q\n", dockerFilePath)
	options := types.ImageBuildOptions{
		SuppressOutput: false,
		Remove:         true,
		ForceRemove:    true,
		PullParent:     true,
		Tags:           []string{"mariohellowebserver"},
		Dockerfile:     "Dockerfile",
	}
	buildResponse, err := dockerClient.ImageBuild(context.Background(), tarDockerfileReader, options)
	if err != nil {
		log.Fatal(err, " :unable to read image build response")
	}
	defer buildResponse.Body.Close()

	termFd, isTerm := term.GetFdInfo(os.Stderr)
	jsonmessage.DisplayJSONMessagesStream(buildResponse.Body, os.Stderr, termFd, isTerm, nil)
}

// GetContainersLogReaders gets our running containers' log readers
func (owned OwnedContainers) GetContainersLogReaders(dockerClient *client.Client) []ContainerReaderStream {

	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Panicf("Unable to list containers. Error: %v", err)
	}

	containerLogStreams := []ContainerReaderStream{}

	for _, container := range containers {
		hostPort := owned[container.ID]
		if hostPort > 0 && container.State == containerRunningStateString {
			readerStream, err := dockerClient.ContainerLogs(context.Background(), container.ID, types.ContainerLogsOptions{
				ShowStdout: true,
				ShowStderr: true,
				Follow:     true,
			})
			if err != nil {
				log.Panicf("Unable to solicit a log reader from the container %s, error: %s\n", container.ID, err)
			}

			containerLogStream := ContainerReaderStream{readerStream, hostPort}
			containerLogStreams = append(containerLogStreams, containerLogStream)
		}
	}

	return containerLogStreams
}

// getContainers lists all the containers running on host machine.
func getContainers(dockerClient *client.Client) ([]types.Container, error) {

	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Printf("Unable to list containers: %v", err)
		return nil, err
	}
	liveContainers := len(containers)
	if liveContainers == 0 {
		log.Println("There are no live containers!")
	}

	return containers, nil
}

// AssertAllContainersAreLive lists all the containers running on host machine
// and asserts that the intended number is live it otherwise panics.
func (owned OwnedContainers) AssertAllContainersAreLive(requestedLiveContainers int, cli *client.Client) error {

	containers, err := getContainers(cli)
	if err != nil {
		log.Panicf("Cannot access live containers...\n")
	}

	containersCount := len(containers)

	if requestedLiveContainers > containersCount {
		log.Panicf("Not enough containers...should be %d containers but found %d.\n", requestedLiveContainers, containersCount)
	}

	// Assert owned containers are running
	for _, container := range containers {
		if owned[container.ID] > 0 && container.State == containerRunningStateString {
			log.Printf("Container %s in %s state.\n", container.ID, container.State)
		} else {
			log.Printf("Found container %s that is not running with state %s, status %s.\n", container.ID, container.State, container.Status)
		}
	}

	return nil
}

// GetContainersStatsReaders gets our running containers' resources readers
func (owned OwnedContainers) GetContainersStatsReaders(dockerClient *client.Client) []ContainerReaderStream {

	containers, err := dockerClient.ContainerList(context.Background(), types.ContainerListOptions{})
	if err != nil {
		log.Panicf("Unable to list containers. Error: %v", err)
	}

	containerStatsStreams := []ContainerReaderStream{}

	for _, container := range containers {
		hostPort := owned[container.ID]
		if hostPort > 0 && container.State == containerRunningStateString {
			out, err := dockerClient.ContainerStats(context.Background(), container.ID, true)
			if err != nil {
				log.Panicf("Unable to solicit a monitoring reader from the container %s, error: %s\n", container.ID, err)
			}

			containerStatsStream := ContainerReaderStream{out.Body, hostPort}
			containerStatsStreams = append(containerStatsStreams, containerStatsStream)
		}
	}

	return containerStatsStreams
}

// StopAllLiveContainers stops as many live containers as possible
func (owned OwnedContainers) StopAllLiveContainers(dockerClient *client.Client) {

	containers, err := getContainers(dockerClient)
	if err == nil {

		for _, container := range containers {
			if owned[container.ID] > 0 {
				err = dockerClient.ContainerStop(context.Background(), container.ID, nil)
				if err != nil {
					log.Printf("Stopping container failed: %v\n", err)
				} else {
					log.Printf("Stopped container with ID: %s\n", container.ID)
				}
				delete(owned, container.ID)
			}
		}
	}
}

// createContainer creates a new container for the dockerImageName
// at the container httpServerContainerPort value.
// and at the host httpServerHostPort value.
// Returns the new container's struct abstraction, error.
// Credit: https://medium.com/tarkalabs/controlling-the-docker-engine-in-go-826012f9671c
func createContainer(dockerClient *client.Client, dockerImageName string, httpServerContainerPort int, httpServerHostPort int) (container.ContainerCreateCreatedBody, error) {

	hostBinding := nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: fmt.Sprintf("%d", httpServerHostPort),
	}
	containerPort, err := nat.NewPort("tcp", fmt.Sprintf("%d", httpServerContainerPort))
	if err != nil {
		log.Panicf("Unable to create a tcp httpServerContainerPort %d\n", httpServerContainerPort)
	}

	portBinding := nat.PortMap{containerPort: []nat.PortBinding{hostBinding}}
	containerBody, err := dockerClient.ContainerCreate(context.Background(),
		&container.Config{Image: dockerImageName},
		&container.HostConfig{
			PortBindings: portBinding,
			AutoRemove:   true,
		},
		nil,
		fmt.Sprintf("HttpServerAt_%d", httpServerHostPort))
	if err != nil {
		log.Panicf("ContainerCreate failed for the image: %s, host port: %d with error: %s\n", dockerImageName, httpServerContainerPort, err)
	}

	return containerBody, err
}

// setContainerLive starts a created container in active live state.
func setContainerLive(dockerClient *client.Client, containerID string) (string, error) {

	err := dockerClient.ContainerStart(context.Background(), containerID, types.ContainerStartOptions{})
	return containerID, err

}

// CreateNewContainer creates a new container and starts it into an active live state for the given dockeImageName.
// at the container httpServerContainerPort value.
// and at the host httpServerHostPort value.
// Returns the new container ID, error.
func setNewContainerLive(dockerClient *client.Client, imageName string, httpServerContainerPort int, httpServerHostPort int) (string, error) {

	cont, err := createContainer(dockerClient, imageName, httpServerContainerPort, httpServerHostPort)
	containerID, err := setContainerLive(dockerClient, cont.ID)
	if err != nil {
		log.Printf("ContainerStart failed for the image: %s, host port: %d with error: %s\n", imageName, httpServerContainerPort, err)
		return "", err
	}
	log.Printf("Container %s with host port %d is live.\n", containerID, httpServerHostPort)
	return containerID, err
}

// CreateContainers requestedLiveContainers containers, creates and starts them into an active live state for the given dockeImageName.
// at the container httpServerContainerPort value.
// and at the host httpServerHostPort value.
func (owned OwnedContainers) CreateContainers(requestedLiveContainers int, dockerClient *client.Client, dockerImageName string, startingListeningHostPort int, containerListeningPort int) {

	for i := 0; i < requestedLiveContainers; i++ {
		hostPort := startingListeningHostPort + i
		containerID, err := setNewContainerLive(dockerClient, dockerImageName, containerListeningPort, hostPort)
		if err != nil {
			log.Fatalf("ContainerCreate failed for the image: %s, host port: %d with error:%s\n", dockerImageName, hostPort, err)
		}
		owned[containerID] = hostPort
	}
}
