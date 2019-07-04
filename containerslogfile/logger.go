package containerslogfile

import (
	"fmt"
	"os"
)

var (
	logFileHandle *os.File
)

// Init opens the requested filesystem logFilePathName with
// os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666 parameters and sets outputs to the file
// and StdErr.
func Init(logFilePathName string) error {

	var err error
	logFileHandle, err = os.OpenFile(logFilePathName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		panic(fmt.Sprintf("Error %s opening logging filename: %v", err, logFilePathName))
	}

	return err
}

// Println calls Output to print to this filesystem log file and stdout.
// Arguments are handled in the manner of fmt.Println.
func Println(v ...interface{}) {
	fmt.Fprintln(logFileHandle, v...)
}

// Close closes the open
func Close() {
	if logFileHandle != nil {
		logFileHandle.Close()
	}
}