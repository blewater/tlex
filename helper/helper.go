// Package helper contains stateless unrelated to the business logic functions.
package helper

import (
	"log"
	"os"
)

// GetCWD returns the current working directory of the executing process i.e. shell pwd
func GetCWD() string {

	dir, err := os.Getwd()
	if err != nil {
		log.Panic(err)
	}

	return dir
}
