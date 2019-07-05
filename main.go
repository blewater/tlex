// Package main is the entrypoint of the tradeline exercise (tlex) artifact.
package main

import (
	"tlex/config"
	wk "tlex/workflow"
)

func main() {

	wk.Workflow(config.GetConfig())

}
