package main

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"tlex/containersapi"
	"tlex/tlextypes"

	containersLogger "tlex/containerslogfile"
	containersStatsLogger "tlex/containerslogfile"

	// Docker
	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/archive"
	"github.com/docker/docker/pkg/jsonmessage"
	"github.com/docker/docker/pkg/term"
	"github.com/docker/go-connections/nat"

	// Contributions
	"github.com/oklog/run"
)

const dockerFilename string = "Dockerfile"
const dockerImageName string = "mariohellowebserver:latest"
const dockerExposedPort int = 8770
const requestedLiveContainers int = 10
const startingHTTPServerNattedPort = 8770
const containerRunningStateString string = "running"
const logFilename string = "containers.log"
const statsFilename string = "containers_stats.log"

// display every modulus throttleStatsInputRequests
const statsPersist bool = false
const statsDisplay bool = false
const throttleStatsInputRequests int = 20

func main() {

	containersLogger.Init(getCWD() + string(os.PathSeparator) + logFilename)
	defer containersLogger.Close()
	containersStatsLogger.Init(getCWD() + string(os.PathSeparator) + statsFilename)
	defer containersStatsLogger.Close()

	ownedContainersID := make(tlextypes.OwnedContainersID)

	ctx := context.Background()
	dockerClient, err := client.NewClientWithOpts(client.FromEnv)
	if err != nil {
		log.Panicf("Docker client.NewClientWithOpts error: %s\n", err)
	}
	dockerClient.NegotiateAPIVersion(ctx)
	defer dockerClient.Close()

	// Image build
	dockerFilePath := getCWD() + string(os.PathSeparator) + dockerFilename
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

	//--------------------------- Container Create

	for i := 0; i < requestedLiveContainers; i++ {
		hostPort := startingHTTPServerNattedPort + i
		containerID, err := createNewContainer(dockerClient, dockerImageName, dockerExposedPort, hostPort)
		if err != nil {
			log.Fatalf("ContainerCreate failed for the image: %s, host port: %d with error:%s\n", dockerImageName, hostPort, err)
		} else {
			ownedContainersID[containerID] = hostPort
		}
	}
	defer stopAllLiveContainers(dockerClient, ownedContainersID)

	assertAllContainersAreLive(dockerClient, ownedContainersID)

	var g run.Group
	monitorContainerStatStreams(dockerClient, &g, ownedContainersID)
	monitorContainerLogStreams(dockerClient, &g, ownedContainersID)
	setupTerminateSignal(&g)
	g.Run()
}

// monitorContainerLogStreams hooks the LOGS streams to single log file, stdout
func monitorContainerLogStreams(dockerClient *client.Client, g *run.Group, ownedContainersID tlextypes.OwnedContainersID) {

	//containersLogReaders := containersapi.GetContainersStatsReaders(dockerClient, ownedContainersID)
	containersLogReaders := containersapi.GetContainersLogReaders(dockerClient, ownedContainersID)
	if len(containersLogReaders) > 0 {

		for _, containersLogReader := range containersLogReaders {

			logReader := containersLogReader.ReaderStream
			hostPort := containersLogReader.HostPort

			g.Add(func() error {
				scanner := bufio.NewScanner(logReader)

				for scanner.Scan() {

					// Strip docker 8 header bytes
					// https://github.com/moby/moby/issues/7375
					text := fmt.Sprintf("@ port %d: %s", hostPort, scanner.Text()[8:])

					log.Println(text)
					containersStatsLogger.Println(text)
				}
				return nil

			}, func(error) {
				logReader.Close()
			})
		}
	}
}

// monitorContainerLogStreams hooks the STATS streams to single log file, stdout
func monitorContainerStatStreams(dockerClient *client.Client, g *run.Group, ownedContainersID tlextypes.OwnedContainersID) {

	containersStatReaders := containersapi.GetContainersStatsReaders(dockerClient, ownedContainersID)
	if len(containersStatReaders) > 0 {

		for _, containersStatReader := range containersStatReaders {

			statsReader := containersStatReader.ReaderStream
			hostPort := containersStatReader.HostPort

			g.Add(func() error {

				decoder := json.NewDecoder(statsReader)
				var containerStats types.Stats
				
				resourceSnapshotCnt := 0
				for err := decoder.Decode(&containerStats); err != io.EOF && err == nil; err = decoder.Decode(&containerStats) {

					if resourceSnapshotCnt % throttleStatsInputRequests == 0 {
						statsBuilder := strings.Builder{}
						statsBuilder.WriteRune('\n')
						statsBuilder.WriteString(fmt.Sprintf("Resource Snaphot %d for http server @ port %d:\n", resourceSnapshotCnt, hostPort))
						statsBuilder.WriteString(fmt.Sprintf("StorageStats.ReadSizeBytes: %v\n", containerStats.StorageStats.ReadSizeBytes))
						statsBuilder.WriteString(fmt.Sprintf("Number of Procs: %v\n", containerStats.NumProcs))
						statsBuilder.WriteString("CPU:\n")
						statsBuilder.WriteString(fmt.Sprintf("Online CPUs: %v\n", containerStats.CPUStats.OnlineCPUs))
						statsBuilder.WriteString(fmt.Sprintf("System Usage: %v\n", containerStats.CPUStats.SystemUsage))
						statsBuilder.WriteString(fmt.Sprintf("Total Usage: %v\n", containerStats.CPUStats.CPUUsage.TotalUsage))
						statsBuilder.WriteString(fmt.Sprintf("UsageInKernelmode: %v\n", containerStats.CPUStats.CPUUsage.UsageInKernelmode))
						statsBuilder.WriteString(fmt.Sprintf("UsageInUsermode: %v\n", containerStats.CPUStats.CPUUsage.UsageInUsermode))
						statsBuilder.WriteString("Memory:\n")
						statsBuilder.WriteString(fmt.Sprintf("Usage: %v\n", containerStats.MemoryStats.Usage))
						statsBuilder.WriteString(fmt.Sprintf("MaxUsage: %v\n", containerStats.MemoryStats.MaxUsage))
						statsBuilder.WriteString(fmt.Sprintf("Limit: %v\n", containerStats.MemoryStats.Limit))
						statsBuilder.WriteString("I/O:\n")
						statsBuilder.WriteString(fmt.Sprintf("Time: %v\n", containerStats.BlkioStats.IoTimeRecursive))
						statsBuilder.WriteString(fmt.Sprintf("Wait Time: %v\n", containerStats.BlkioStats.IoWaitTimeRecursive))
						statsBuilder.WriteString(fmt.Sprintf("Serviced: %v\n", containerStats.BlkioStats.IoServicedRecursive))
						statsBuilder.WriteString(fmt.Sprintf("Service Bytes: %v\n", containerStats.BlkioStats.IoServiceBytesRecursive))
						statsBuilder.WriteString(fmt.Sprintf("Queued: %v\n", containerStats.BlkioStats.IoQueuedRecursive))					

						statsString := statsBuilder.String()
						log.Println(statsString)
						containersLogger.Println(statsString)
					}

					resourceSnapshotCnt++
				}

				return nil

			}, func(error) {

				statsReader.Close()

			})
		}
	}
}

// setupTerminateSignal connects the os.Interrupt signal to a quit channel to
// start teardown for this process.
func setupTerminateSignal(g *run.Group) {

	log.Println()
	log.Println()
	log.Println("Press Ctrl-C at any point to exit.")
	log.Println("----------------------------------")
	log.Println()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt)
	g.Add(func() error {
		<-quit
		log.Println()
		log.Println()
		log.Println("Received Interrupt signal. Cleaning up and exiting")
		log.Println("--------------------------------------------------")
		log.Println()
		return nil
	}, func(err error) {
		close(quit)
	})
}

func getCWD() string {

	dir, err := os.Getwd()
	if err != nil {
		log.Fatal(err)
	}

	return dir
}

// stopAllLiveContainers stops as many live containers as possible
func stopAllLiveContainers(cli *client.Client, ownedContainers tlextypes.OwnedContainersID) {

	containers, err := getContainers(cli)
	if err == nil {

		for _, container := range containers {
			if ownedContainers[container.ID] > 0 {
				err = cli.ContainerStop(context.Background(), container.ID, nil)
				if err != nil {
					log.Printf("Stopping container failed: %v\n", err)
				} else {
					log.Printf("Stopped container with ID: %s\n", container.ID)
				}
			}
		}
	}
}

// PruneContainers is a debugging and unsuitable production function:
// it clears all containers that are not running.
func pruneContainers() error {

	cli, err := client.NewEnvClient()
	if err != nil {
		log.Fatalf("Unable to create docker client in pruneContainers() error: %v\n", err)
	}

	report, err := cli.ContainersPrune(context.Background(), filters.Args{})
	if err != nil {
		log.Printf("Prune containers failed: %v\n", err)
		return err
	}
	log.Printf("Containers pruned: %v\n", report.ContainersDeleted)
	return nil
}

// CreateNewContainer creates a new container for the custom localhost web server image
// with the Docker exposed webServerContainerPort
// mapping to the host webServerMappedPort.
// Returns the new container ID, error.
// Credit: https://medium.com/tarkalabs/controlling-the-docker-engine-in-go-826012f9671c
func createNewContainer(cli *client.Client, imageName string, httpServerContainerPort int, httpServerHostPort int) (string, error) {

	hostBinding := nat.PortBinding{
		HostIP:   "0.0.0.0",
		HostPort: fmt.Sprintf("%d", httpServerHostPort),
	}
	containerPort, err := nat.NewPort("tcp", fmt.Sprintf("%d", httpServerContainerPort))
	if err != nil {
		log.Printf("Unable to create a tcp httpServerContainerPort %d\n", httpServerContainerPort)
		return "", err
	}

	portBinding := nat.PortMap{containerPort: []nat.PortBinding{hostBinding}}
	cont, err := cli.ContainerCreate(context.Background(), &container.Config{
		Image: imageName,
	},
		&container.HostConfig{
			PortBindings: portBinding,
			AutoRemove:   true,
		},
		nil,
		fmt.Sprintf("HttpServerAt_%d", httpServerHostPort))
	if err != nil {
		log.Printf("ContainerCreate failed for the image: %s, host port: %d with error: %s\n", imageName, httpServerContainerPort, err)
		return "", err
	}

	err = cli.ContainerStart(context.Background(), cont.ID, types.ContainerStartOptions{})
	if err != nil {
		log.Printf("ContainerStart failed for the image: %s, host port: %d with error: %s\n", imageName, httpServerContainerPort, err)
		return "", err
	}
	log.Printf("Container %s with host port %d is live.\n", cont.ID, httpServerHostPort)
	return cont.ID, nil
}

// getContainers lists all the containers running on host machine
// credit: https://medium.com/tarkalabs/controlling-the-docker-engine-in-go-826012f9671c
func getContainers(cli *client.Client) ([]types.Container, error) {

	containers, err := cli.ContainerList(context.Background(), types.ContainerListOptions{})
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

// assertAllContainersAreLive lists all the containers running on host machine
// and asserts that the intended number is live it otherwise panics.
func assertAllContainersAreLive(cli *client.Client, ownedContainers tlextypes.OwnedContainersID) error {

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
		if ownedContainers[container.ID] > 0 && container.State == containerRunningStateString {
			log.Printf("Container %s in %s state.\n", container.ID, container.State)
		} else {
			log.Printf("Found container %s that is not running with state %s, status %s.\n", container.ID, container.State, container.Status)
		}
	}

	return nil
}
