// Package dockerapi is the facade to the Docker remote api.
package dockerapi

import (
	"sync"
	"context"
	"fmt"
	"io"
	"log"
	"os"
	"tlex/mapsi2disk"

	"golang.org/x/sync/errgroup"
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

// Cleanup previous owned live instances that might have been left hanging.
func RemoveLiveContainersFromPreviousRun() {

	readObj, err := mapsi2disk.ReadContainerPortsFromDisk(mapsi2disk.GobFilename)
	readBackOwnedContainers := readObj.(map[string]int)

	if err == nil {
		defer mapsi2disk.DeleteFile(mapsi2disk.GobFilename)

		dockerClient := GetDockerClient()

		for containerID := range readBackOwnedContainers {
			log.Printf("Deleting container: %v from previous launch.\n", containerID)
			err = dockerClient.ContainerStop(context.Background(), containerID, nil)
		}
	}
}

// GetDockerClient returns a docker remote api client handle value foundational to all Docker remote api interactions.
// Upon error it panics.
// This process creates a docker client when launching and holds on to it for all API interactions.
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

// BuildDockerImage builds a Docker Image for a given dockerFilePath located in the same folder
// of the running process.
// Upon Error it exits process.
func BuildDockerImage(dockerClient *client.Client, dockerFilePath string) {

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

// GetContainersLogReaders gets our running containers' log readers.
// Upon failure, it panics.
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

	return containers, nil
}

// CleanLeftOverContainers stops any *owned* live containers.
// Useful in during lauching of containers fails and have to clean up launched instances.
func (owned OwnedContainers)CleanLeftOverContainers(dockerClient *client.Client) {

	containers, err := getContainers(dockerClient)
	if err == nil {
		for _, container := range containers {
			if _, ok := owned[container.ID]; ok {
				err = dockerClient.ContainerStop(context.Background(), container.ID, nil)
			}
		}
	}
}

// AssertOwnedContainersAreLive lists all the containers running on the host
// and asserts
// 1. Existence of enough live containers
// 2. This process' owned containers are live.
// It panics otherwise.
func (owned OwnedContainers) AssertOwnedContainersAreLive(requestedLiveContainers int, cli *client.Client) error {

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
			log.Panicf("Found container %s that is not running with state %s, status %s.\n", container.ID, container.State, container.Status)
		}
	}

	return nil
}

// AssertRequestedContainersAreLive lists all the containers running on the host
// and asserts that the intended containers number is live otherwise it panics.
// This is called by tests. AssertOwnedContainersAreLive is called by default within workflow
func AssertRequestedContainersAreLive(requestedLiveContainers int) {

	cli := GetDockerClient()

	containers, err := getContainers(cli)
	if err != nil {
		log.Panicf("Cannot access live containers...\n")
	}

	containersCount := len(containers)

	if requestedLiveContainers > containersCount {
		log.Panicf("Not enough containers...Wanted %d containers but got %d.\n", requestedLiveContainers, containersCount)
	}

	// Assert owned containers are running
	for _, container := range containers {
		if container.State == containerRunningStateString {
			log.Printf("Container %s in %s state.\n", container.ID, container.State)
		} else {
			log.Panicf("Found container %s that is not running with state %s, status %s.\n", container.ID, container.State, container.Status)
		}
	}

	fmt.Printf("\n**** Passed RequestedContainers == Live assertion. ****\n\n")
}

// AssertRequestedContainersAreGone check that no containers exist and that the system cleaned up otherwise it panics.
// This is called by tests.
func AssertRequestedContainersAreGone() {

	cli := GetDockerClient()

	containers, err := getContainers(cli)
	if err != nil {
		log.Panicf("Cannot access live containers...\n")
	}

	containersCount := len(containers)

	if containersCount > 0 {
		log.Printf("Containers are still alive... Wanted 0 containers but got %d.\n", containersCount)
	}

	// Assert owned containers are not running
	for _, container := range containers {
		log.Panicf("Found container %s with state %s, status %s.\n", container.ID, container.State, container.Status)
	}

	fmt.Printf("\n**** Passed RequestedContainers == 0 assertion. ****\n\n")
}

// GetContainersStatsReaders gets our running containers' resources readers
// Upon error it panics.
func (owned OwnedContainers) GetContainersStatsReaders(dockerClient *client.Client) []ContainerReaderStream {

	dockerClient = GetDockerClient()
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
func (owned OwnedContainers) StopAllLiveContainers(terminatorGroup *sync.WaitGroup, dockerClient *client.Client) {

	containers, err := getContainers(dockerClient)
	if err == nil {

		for _, container := range containers {
			if owned[container.ID] > 0 {

				contID := container.ID

				terminatorGroup.Add(1)

				go func() {
					err = dockerClient.ContainerStop(context.Background(), contID, nil)
					if err != nil {
						log.Printf("Stopping container failed: %v\n", err)
					} else {
						log.Printf("Stopped container with ID: %s\n", contID)
					}
					defer terminatorGroup.Done()
				}()
			}
		}
	}
}

// createContainer creates a new container for the dockerImageName
// at the container httpServerContainerPort value.
// and at the host httpServerHostPort value.
// Returns the new container's struct abstraction, error.
// Upon error it panics.
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

// CreateNewContainer
// 1. creates a new container for the given dockeImageName and
// 2. starts it into an active live state:
// at the container httpServerContainerPort value,
// and at the host httpServerHostPort value.
// Returns the new container ID, error.
// Upon Error it panics.
func setNewContainerLive(dockerClient *client.Client, imageName string, httpServerContainerPort int, httpServerHostPort int) (string, error) {

	cont, err := createContainer(dockerClient, imageName, httpServerContainerPort, httpServerHostPort)
	containerID, err := setContainerLive(dockerClient, cont.ID)
	if err != nil {
		log.Panicf("ContainerStart failed for the image: %s, host port: %d with error: %s\n", imageName, httpServerContainerPort, err)
		return "", err
	}
	log.Printf("Container %s with host port %d is live.\n", containerID, httpServerHostPort)
	return containerID, err
}

// CreateContainers requests live containers. It creates and starts them into an active live state for the given dockeImageName.
// at the container httpServerContainerPort value.
// and at the host httpServerHostPort value.
// Upon error it panics.
func (owned OwnedContainers) CreateContainers(launcherGroup *errgroup.Group, requestedLiveContainers int, dockerClient *client.Client, dockerImageName string, startingListeningHostPort int, containerListeningPort int) {

	// Manage concurrent access to shared owned map
	ownedMutex := &sync.Mutex{}

	for i := 0; i < requestedLiveContainers; i++ {

		// necessary to capture each loop iteration of i
		portCounter := i

		// Concurrent launching of docker instances
		launcherGroup.Go(func() error {

			hostPort := startingListeningHostPort + portCounter
			containerID, err := setNewContainerLive(dockerClient, dockerImageName, containerListeningPort, hostPort)
			if err != nil {
				log.Panicf("ContainerCreate failed for the image: %s, host port: %d with error:%s\n", dockerImageName, hostPort, err)
			}

			ownedMutex.Lock()
			owned[containerID] = hostPort
			ownedMutex.Unlock()

			return err
		})
	}
}

// PersistOpenContainers saves the presumed populated owned containers map id-> ports into the filesystem.
func (owned OwnedContainers) PersistOpenContainerIDs() {

	mapToSave := map[string]int(owned)

	err := mapsi2disk.SaveContainerPorts2Disk(mapsi2disk.GobFilename, &mapToSave)
	if err != nil {
		log.Printf("SaveContainerPorts2Disk() error = %v\n", err)
	}

}