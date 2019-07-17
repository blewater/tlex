// Package mapsi2disk persists the containers port map struct to the gobFileName
// and allows reading it again and deleting the file.
package mapsi2disk

import (
	"encoding/gob"
	"fmt"
	"log"
	"os"
)

// GobFile is the file system sink for a map of container ids
type GobFile struct {
	file *os.File
}

// GobFilename is the filename without path of the gob file.
const GobFilename = "ids.gob"

// Open creates or opens the requested filesystem logFilePathName with the ANDed flags.
func (gobFile *GobFile) open(gobFullFilenamePath string, osReadWriteFlag int) error {

	var err error
	gobFile.file, err = os.OpenFile(gobFullFilenamePath, osReadWriteFlag, 0666)

	return err
}

// SaveContainerPorts2Disk saves an encoded (serialized) ownedContainers map object to disk.
// gobFullFilenamePath is the full name + path of the gob file.
func SaveContainerPorts2Disk(gobFullFilenamePath string, ownedContainers interface{}) error {

	gobFile := &GobFile{}
	err := gobFile.open(gobFullFilenamePath, os.O_WRONLY|os.O_TRUNC|os.O_CREATE)
	if err == nil {
		err = gobFile.encContainerIds2Disk(ownedContainers)

		defer gobFile.file.Close()
	}

	return err
}

// ReadContainerPortsFromDisk reads an decoded (serialized) ownedContainers map object from disk.
// gobFullFilenamePath is the full name + path of the gob file.
func ReadContainerPortsFromDisk(gobFullFilenamePath string) (interface{}, error) {

	gobFile := &GobFile{}
	err := gobFile.open(gobFullFilenamePath, os.O_RDONLY)

	var ownedContainers interface{} = make(map[string]int)

	if err != nil {

		log.Println(fmt.Sprintf("No trace of live containers from previous launch: %v.", err))

	} else {

		ownedContainers, err = gobFile.decContainerIdsFromDisk()

		defer gobFile.file.Close()
	}

	return ownedContainers, err
}

func (gobFile *GobFile) encContainerIds2Disk(objToSave interface{}) error {

	enc := gob.NewEncoder(gobFile.file)

	mapToSave := objToSave.(*map[string]int)

	err := enc.Encode(&mapToSave)
	if err != nil {
		log.Printf("Error when encoding/saving objToSave: %v\n", err)
	}
	return err
}

func (gobFile *GobFile) decContainerIdsFromDisk() (interface{}, error) {

	mapToRead := make(map[string]int)

	enc := gob.NewDecoder(gobFile.file)
	err := enc.Decode(&mapToRead)
	if err != nil {
		log.Printf("Error when decoding/reading mapToRead: %v\n", err)
	}
	return mapToRead, err
}

// DeleteFile removes the "closed" file.
func DeleteFile(gobFullFilenamePath string) error {

	// delete file
	var err = os.Remove(gobFullFilenamePath)
	if err != nil {
		log.Printf("Error: %v when deleting filename: %v\n", err, gobFullFilenamePath)
	}

	return err
}
