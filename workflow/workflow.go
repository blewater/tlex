// Package workflow holds the state machine + specific requirements fulfillment.
package workflow

import (
	"bufio"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"os/signal"
	"strings"
	"syscall"

	"tlex/config"
	"tlex/dockerapi"
	"tlex/logger"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
	"github.com/oklog/run"
)

/** These package variables facilitate testing by state sharing **/

// ContainersLaunched signifies containers are live.
type ContainersLaunched chan bool

// ContainersRemoved signifies containers are removed.
type ContainersRemoved chan bool

// ContainersChecked signifies containers' health check completed.
type ContainersChecked chan bool

// completedLaunch with 1 capacity set to true when containers are live
var containersLaunched = make(ContainersLaunched, 1)
var containersRemoved = make(ContainersRemoved, 1)
var containersChecked = make(ContainersChecked, 1)

// goroutines manager
var g run.Group

// Workflow performs the necessary steps to accomplish this tool's purpose.
func Workflow(cfg config.AppConfig) {

	// Step 0: Facade to the docker remote API
	dumpConfig(cfg)
	var dockerClient = dockerapi.GetDockerClient()
	defer dockerClient.Close()

	// Step 1: Build the Docker Image.
	dockerapi.BuildDockerImage(dockerClient, cfg.DockerFilename)

	// Step 2: Create the live Docker Containers.
	ownedContainers := make(dockerapi.OwnedContainers)
	ownedContainers.CreateContainers(cfg.RequestedLiveContainers, dockerClient, cfg.DockerImageName, cfg.StartingHTTPServerNattedPort, cfg.DockerExposedPort)

	// Step 3: Assume all containers are live.
	ownedContainers.AssertOwnedContainersAreLive(cfg.RequestedLiveContainers, dockerClient)
	if cfg.InTestingModeWithChannelsSync {
		containersLaunched <- true
	}

	// Step 4: Monitor stats.
	monitorContainerStatStreams(logger.GetLogger(cfg.StatsFilename), dockerClient, cfg, &g, ownedContainers)

	// Step 5: Aggregate the containers logs.
	aggContainersLogStreams(logger.GetLogger(cfg.LogFilename), dockerClient, cfg, &g, ownedContainers)

	// Step 6: Hook a clean exit sequence to the interrupt signal.
	setupTerminateSignal(&g, cfg)

	// Exit concurrent flow when 4, 5, 6 exit or err out.
	g.Run()

	removeContainers(cfg, ownedContainers, dockerClient)
}

func dumpConfig(cfg config.AppConfig) {

	log.Printf("\nApplication Configuration:\n%v\n\n", cfg)
}

// SetWorkflowSyncWithAPI makes workflow cancellable with channel events
// Useful for unit testing, benchmarkiing scenarios among others.
func SetWorkflowSyncWithAPI() (ContainersLaunched, ContainersRemoved, ContainersChecked) {

	g.Add(func() error {

		<-containersChecked

		return nil

	}, func(err error) {

	})

	return containersLaunched, containersRemoved, containersChecked
}

// setupTerminateSignal connects the os.Interrupt signal to a quit channel to
// start teardown for this process.
func setupTerminateSignal(g *run.Group, cfg config.AppConfig) {

	// No point to wait for 0 containers
	if cfg.RequestedLiveContainers == 0 {
		return
	}

	log.Println()
	log.Println()
	log.Println("Press Ctrl-C at any point to exit.")
	log.Println("----------------------------------")
	log.Println()

	quit := make(chan os.Signal, 1)
	// Honor interrupt / kill signals
	signal.Notify(quit, os.Interrupt, syscall.SIGKILL, syscall.SIGTERM, syscall.SIGQUIT)
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

// aggContainersLogStreams aggregates the LOGS streams to the single log file, stdout
func aggContainersLogStreams(containersLogger logger.Logger, dockerClient *client.Client, cfg config.AppConfig, g *run.Group, ownedContainers dockerapi.OwnedContainers) {

	containersLogReaders := ownedContainers.GetContainersLogReaders(dockerClient)
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
					containersLogger.Println(text)
				}

				return nil

			}, func(error) {

				// defer close equivalent
				logReader.Close()

			})
		}
	}
}

// monitorContainerStatStreams aggregates the STATS streams to single log file (optional), stdout
func monitorContainerStatStreams(containersStatsLogger logger.Logger, dockerClient *client.Client, cfg config.AppConfig, g *run.Group, ownedContainers dockerapi.OwnedContainers) {

	const Bytes2MiB float64 = 1024 * 1024
	const Bytes2GiB float64 = Bytes2MiB * 1024

	containersStatReaders := ownedContainers.GetContainersStatsReaders(dockerClient)
	if len(containersStatReaders) > 0 {

		for _, containersStatReader := range containersStatReaders {

			statsReader := containersStatReader.ReaderStream
			hostPort := containersStatReader.HostPort

			g.Add(func() error {

				decoder := json.NewDecoder(statsReader)
				var stats types.Stats

				resourceSnapshotCnt := 0
				for err := decoder.Decode(&stats); err != io.EOF && err == nil; err = decoder.Decode(&stats) {

					if cfg.StatsDisplay && resourceSnapshotCnt%cfg.ThrottleStatsInputRequests == 0 {
						statsBuilder := strings.Builder{}
						statsBuilder.WriteRune('\n')
						statsBuilder.WriteString(fmt.Sprintf("Resource Snaphot %d for http server @ port %d, PIDs:%d\n", resourceSnapshotCnt, hostPort, stats.PidsStats.Current))
						var cpuPercent float64
						if stats.CPUStats.SystemUsage != 0 {
							cpuPercent = (float64(stats.CPUStats.CPUUsage.TotalUsage) / float64(stats.CPUStats.SystemUsage)) * float64(len(stats.CPUStats.CPUUsage.PercpuUsage)) * 100.0
						}
						statsBuilder.WriteString(fmt.Sprintf("CPU -> CPU %.2f%%, CPUs: %v, Usage Total: %v, System: %v\n", cpuPercent, stats.CPUStats.OnlineCPUs, stats.CPUStats.CPUUsage.TotalUsage, stats.CPUStats.SystemUsage))
						var memPercent float64
						if stats.MemoryStats.Limit != 0 {
							memPercent = float64(stats.MemoryStats.Usage) / float64(stats.MemoryStats.Limit) * 100.0
						}
						statsBuilder.WriteString(fmt.Sprintf("Memory -> %.2f%% Usage: %.2fMiB, MaxUsage: %.2fMiB, Limit: %.2fGiB\n", memPercent, float64(stats.MemoryStats.Usage)/Bytes2MiB, float64(stats.MemoryStats.MaxUsage)/Bytes2MiB, float64(stats.MemoryStats.Limit)/Bytes2GiB))
						statsBuilder.WriteString(fmt.Sprintf("IO -> StorageStats.ReadSizeBytes: %v, Time: %v, Wait Time: %v, Serviced: %v, Service Bytes: %v, Queued: %v\n", stats.StorageStats.ReadSizeBytes, stats.BlkioStats.IoTimeRecursive, stats.BlkioStats.IoWaitTimeRecursive, stats.BlkioStats.IoServicedRecursive, stats.BlkioStats.IoServiceBytesRecursive, stats.BlkioStats.IoQueuedRecursive))

						statsString := statsBuilder.String()
						log.Println(statsString)
						if cfg.StatsPersist {
							containersStatsLogger.Println(statsString)
						} else {
							log.Printf("\nStats in non-persistence mode.\n")
						}
					}

					resourceSnapshotCnt++
				}

				return nil

			}, func(error) {

				// defer close equivalent
				statsReader.Close()

			})
		}
	}
}

// removeContainers removes the containers from the domain engine. Intended as a late clean up
// step in the workflow before shutting down.
func removeContainers(cfg config.AppConfig, ownedContainers dockerapi.OwnedContainers, dockerClient *client.Client) {

	ownedContainers.StopAllLiveContainers(dockerClient)

	//*** Note if InTestingModeWithChannelsSync is set to true during
	// normal operation it will wait on containersChecked after erasing the containers.
	// Used only for unit tests requiring being notified for intermediate state changes.

	if cfg.InTestingModeWithChannelsSync {
		containersRemoved <- true
		<-containersChecked
	}

}
