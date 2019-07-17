// Package mapsi2disk persists the containers port map struct to the gobFileName
// and allows reading it again and deleting the file.
package mapsi2disk

import (
	"testing"
	"fmt"
)

func Test_SaveMapToDiskAndDelete(t *testing.T) {
	mapToSave := make(map[string]int)
	mapToSave["Test"] = 8888

	filename := "./testdata/" + GobFilename
	err := SaveContainerPorts2Disk(filename, &mapToSave)
	if err != nil {
		t.Errorf("SaveMapToDisk() error = %v", err)
	}
	err = DeleteFile(filename)
	if err != nil {
		t.Errorf("DeleteToDisk() error = %v", err)
	}
}

func Test_AssertSavingIsWhatIsRead(t *testing.T) {
	mapToSave := make(map[string]int)
	mapToSave["Test1"] = 8888
	mapToSave["Test2"] = 8889
	mapToSave["Test3"] = 8890

	// Save
	filename := "./testdata/" + GobFilename
	err := SaveContainerPorts2Disk(filename, &mapToSave)
	if err != nil {
		t.Errorf("SaveMapToDisk() error = %v", err)
	}

	// Read
	readObj, err := ReadContainerPortsFromDisk(filename)
	readBackOwnedContainers := readObj.(map[string]int)
	if err != nil {
		t.Errorf("ReadMapFromDisk() error = %v", err)
	}
	for i := 1; i <= 3; i++ {

		testName := fmt.Sprintf("Test%v", i)
		savedPort := mapToSave[testName]
		readAgainPort := readBackOwnedContainers[testName]
		if savedPort != readAgainPort {
			t.Errorf("savedPort: %v != readAgainPort: %v\n", savedPort, readAgainPort)
		}
	}
	
	// Delete
	err = DeleteFile(filename)
	if err != nil {
		t.Errorf("DeleteToDisk() error = %v", err)
	}
}
func Test_SaveMapToDiskTwiceAndDelete(t *testing.T) {
	mapToSave := make(map[string]int)
	mapToSave["Test"] = 8888

	filename := "./testdata/" + GobFilename
	err := SaveContainerPorts2Disk(filename, &mapToSave)
	if err != nil {
		t.Errorf("SaveMapToDisk() error = %v", err)
	}
	err = SaveContainerPorts2Disk(filename, &mapToSave)
	if err != nil {
		t.Errorf("DeleteToDisk() error = %v", err)
	}
	err = DeleteFile(filename)
	if err != nil {
		t.Errorf("DeleteToDisk() error = %v", err)
	}
}

func Test_AssertReadWithoutSavedFileProducesError(t *testing.T) {

	// Delete
	filename := "./testdata/" + GobFilename
	// Delete
	err := DeleteFile(filename)
	if err == nil {
		t.Errorf("DeleteToDisk() of non-existent file did not produce error = %v", err)
	}

	// Read
	_, err = ReadContainerPortsFromDisk(filename)
	if err == nil {
		t.Errorf("ReadMapFromDisk() did not produce an error for a non-existent file = %v", err)
	}
}