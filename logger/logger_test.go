// Package logger abstracts the filesystem file logging creation specifics + text file reference + light api.
package logger

import (
	"fmt"
	"os"
	"testing"
	"tlex/helper"
)

// TestGetLogger tests the successful creation of a new log file with 0 size
func TestGetLogger(t *testing.T) {

	logFilename := helper.GetCWD() + string(os.PathSeparator) + "testdata" + string(os.PathSeparator) + "containers.log"

	// ---
	// Start Clean
	os.Remove(logFilename) // ignore errors

	fmt.Println("==> done deleting file")

	// ---
	// getLogger should create a file with 0 size
	logger := GetLogger(logFilename)

	logger.Close()

	// ---
	// open same file and check size
	file, err := os.Open(logFilename)
	if err != nil {
		t.Errorf("Error in opening the same file that getLogger() created: %v\n", err)
	}
	fi, err := file.Stat()
	if err != nil {
		t.Errorf("Error in accessing stats for the same file that getLogger() created: %v\n", err)
	}
	sz := fi.Size()
	if sz != 0 {
		t.Errorf("Size %v is not 0 for the new log file of getLogger()\n", sz)
	}
	file.Close()
}