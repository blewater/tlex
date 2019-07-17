// Package main is the entrypoint of the tradeline exercise (tlex) artifact.
package main

import (
	"tlex/config"
	"tlex/dockerapi"
	wk "tlex/workflow"
)

// Cleanup previous owned live instances that might have been left hanging.
func init() {

	dockerapi.RemoveLiveContainersFromPreviousRun()
}

func main() {

	wk.Workflow(config.GetConfig())

}
