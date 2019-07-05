// Package workflow holds the state machine + specific requirements fulfillment.
package workflow

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"testing"
	"time"
	"tlex/config"
	"tlex/dockerapi"
	"tlex/helper"
)

func intro(cfg *config.AppConfig, requestedLiveContainers int) {

	cfg.RequestedLiveContainers = requestedLiveContainers
	fmt.Printf("Requested %d containers.\n", cfg.RequestedLiveContainers)
	cfg.LogFilename = helper.GetCWD() + string(os.PathSeparator) + "testdata" + string(os.PathSeparator) + "containers.log"
	cfg.StatsFilename = helper.GetCWD() + string(os.PathSeparator) + "testdata" + string(os.PathSeparator) + "containers_stats.log"
	cfg.DockerFilename = helper.GetCWD() + string(os.PathSeparator) + "testdata" + string(os.PathSeparator) + "Dockerfile"
	cfg.InTestingModeWithChannelsSync = true
}

func testWorkflowXInstancesAppConfig(cfg *config.AppConfig) {

	intro(cfg, cfg.RequestedLiveContainers)

	containersLaunched, containersRemoved, containersChecked := SetWorkflowSyncWithAPI()

	requestedLiveContainers := cfg.RequestedLiveContainers

	go func() {

		<-containersLaunched
		dockerapi.AssertRequestedContainersAreLive(requestedLiveContainers)
		containersChecked <- true
		<-containersRemoved
		dockerapi.AssertRequestedContainersAreGone()
		containersChecked <- true
	}()

	Workflow(*cfg)
}

func testWorkflowXInstances(requestedLiveContainers int) {

	cfg := config.GetConfig()
	cfg.RequestedLiveContainers = requestedLiveContainers
	testWorkflowXInstancesAppConfig(&cfg)
}

// go test -run Test_Continuous_Logs_Http_Requests_100_Containers -timeout 100000s
func Test_Continuous_Logs_Http_Requests_100_Containers(t *testing.T) {

	cfg := config.GetConfig()
	cfg.StatsDisplay = true
	cfg.StatsPersist = true
	cfg.ThrottleStatsInputRequests = 1

	var numberOfContainers uint64 = 100

	// Start instances
	intro(&cfg, int(numberOfContainers))

	go func() {
		// 15 secs to get up
		time.Sleep(30 * time.Second)
		start := time.Now()
		var firstPort uint64 = 8770

		var i uint64

		// As long as it last before the timeout
		for i = 0; ; i++ {
			nextPort := firstPort + (i % numberOfContainers)
			url := fmt.Sprintf("http://localhost:%d/request%d/at port %d/", nextPort, i+1, nextPort)
			err := fetch(url)
			if err != nil {
				t.Errorf("Error %s\n", err)
			}
		}
		fmt.Printf("%.2fs elapsed\n", time.Since(start).Seconds())
	}()

	Workflow(cfg)

}

func fetch(url string) error {

	start := time.Now()
	resp, err := http.Get(url)

	if err != nil {
		fmt.Printf("http error: %v for %v\n", err, url)
		return err
	}
	nbytes, err := io.Copy(ioutil.Discard, resp.Body)
	resp.Body.Close() // don't leak resources

	if err != nil {
		fmt.Print(fmt.Sprintf("while reading %s: %v\n", url, err))
		return err
	}
	secs := time.Since(start).Seconds()
	fmt.Printf("%.2fs %7d %s\n", secs, nbytes, url)
	return err
}

// TestWorkflow0Containers tests the workflow for 0 requested containers.
func Test_Workflow_0_Containers(t *testing.T) {

	cfg := config.GetConfig()
	intro(&cfg, 0)

	containersLaunched, containersRemoved, containersChecked := SetWorkflowSyncWithAPI()

	requestedLiveContainers := cfg.RequestedLiveContainers

	go func() {

		<-containersLaunched
		dockerapi.AssertRequestedContainersAreLive(requestedLiveContainers)
		containersChecked <- true
		<-containersRemoved
		dockerapi.AssertRequestedContainersAreGone()
		containersChecked <- true
	}()

	Workflow(cfg)
}

func Test_Workflow_3_Containers_No_Stats(t *testing.T) {

	cfg := config.GetConfig()
	cfg.StatsPersist = false
	cfg.StatsDisplay = false
	cfg.RequestedLiveContainers = 3
	testWorkflowXInstancesAppConfig(&cfg)
}

func Test_Workflow_1_Containers(t *testing.T) {

	testWorkflowXInstances(1)

}

func Test_Workflow_5_Containers(t *testing.T) {

	testWorkflowXInstances(5)

}

// go test -run Test_Workflow_20_Container -timeout 100s
func Test_Workflow_20_Containers(t *testing.T) {

	testWorkflowXInstances(20)

}
