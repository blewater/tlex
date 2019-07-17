// Package logger abstracts the filesystem file logging creation specifics + text file reference + light api.
package logger

import (
	"fmt"
	"os"
)

// Logger is the file system sink for log messages.
type Logger struct {
	logFile *os.File
}

// GetLogger returns a new logger for a given name in the process working directory.
func GetLogger(logFilename string) Logger {

	logger := Logger{}
	logger.open(logFilename)

	return logger
}

// Open creates/opens the requested filesystem logFilePathName with
// os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666 parameters.
func (logger *Logger) open(logFilePathName string) {

	var err error
	logger.logFile, err = os.OpenFile(logFilePathName, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0666)

	if err != nil {
		panic(fmt.Sprintf("Error %s opening logging filename: %v", err, logFilePathName))
	}

}

// Println calls Output to print to this filesystem log file and stdout.
// Arguments are handled in the manner of fmt.Println.
func (logger *Logger) Println(v ...interface{}) {
	fmt.Fprintln(logger.logFile, v...)
}

// Close closes the open file handle
func (logger *Logger) Close() {
	logger.logFile.Close()
}
