package tlextypes

import "io"

// OwnedContainersID contains the containers ID created by this process
type OwnedContainersID map[string]int

// ContainerReaderStream contains a container reader stream, host port mapped to the container http server.
type ContainerReaderStream struct {
	ReaderStream io.ReadCloser
	HostPort     int
}

// type ContainerStats struct {
//     ID       string `json:"id"`
//     Read     string `json:"read"`
//     Preread  string `json:"preread"`
//     CPUStats cpu `json:"cpu_stats"`
// }
