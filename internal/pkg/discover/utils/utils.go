// Copyright 2022 Metrika Inc.
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
// http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package utils

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"net/http"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	"github.com/docker/docker/client"
	"github.com/joho/godotenv"
)

var (
	// ErrContainerNotFound container not found
	ErrContainerNotFound = errors.New("container not found")

	// ErrEmptyLogFile log file is empty
	ErrEmptyLogFile = errors.New("log file is empty")

	// DefaultDockerHost host docker daemon address to connect to
	DefaultDockerHost = ""

	// DefaultDockerAdapter default docker adapter for container discovery.
	DefaultDockerAdapter = DockerAdapter(&DockerProductionAdapter{climu: &sync.Mutex{}})
)

// DockerAdapter container discovery interface.
type DockerAdapter interface {
	// GetRunningContainers returns a slice of all
	// currently running Docker containers
	GetRunningContainers() ([]dt.Container, error)

	// MatchContainer takes a slice of containers and regex strings.
	// It returns the first running container to match any of the identifiers.
	// If no matches are found, ErrContainerNotFound is returned.
	MatchContainer(containers []dt.Container, identifiers []string) (dt.Container, error)

	DockerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error)

	DockerEvents(ctx context.Context, options types.EventsOptions) (
		<-chan events.Message, <-chan error, error)

	Close() error
}

// DockerProductionAdapter adapter for accessing the host docker daemon
type DockerProductionAdapter struct {
	cli   *client.Client
	climu *sync.Mutex
}

// Close closes the underlying http connection
func (a *DockerProductionAdapter) Close() error {
	a.climu.Lock()
	defer a.climu.Unlock()

	if a.cli != nil {
		cli := a.cli
		a.cli = nil
		return cli.Close()
	}
	return nil
}

func (a *DockerProductionAdapter) resetClient() error {
	cli, err := getDockerClient()
	if err != nil {
		a.cli = nil

		return err
	}
	a.cli = cli
	return nil
}

// GetRunningContainers returns a slice of all
// currently running Docker containers
func (a *DockerProductionAdapter) GetRunningContainers() ([]dt.Container, error) {
	a.climu.Lock()
	if a.cli == nil {
		if err := a.resetClient(); err != nil {
			return nil, err
		}
	}
	a.climu.Unlock()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containers, err := a.cli.ContainerList(ctx, dt.ContainerListOptions{})
	if err != nil {
		if !errors.Is(ErrContainerNotFound, err) {
			a.climu.Lock()
			a.cli.Close()
			a.cli = nil
			a.climu.Unlock()
		}
		return nil, err
	}

	return containers, nil
}

// MatchContainer takes a slice of containers and regex strings.
// It returns the first running container to match any of the identifiers.
// If no matches are found, ErrContainerNotFound is returned.
func (a *DockerProductionAdapter) MatchContainer(containers []dt.Container, identifiers []string) (dt.Container, error) {
	for _, container := range containers {
		for _, rStr := range identifiers {
			r, err := regexp.Compile(rStr)
			if err != nil {
				return dt.Container{}, err
			}

			// Try to match the identifier with container names
			for _, name := range container.Names {
				if r.MatchString(name) {
					return container, nil
				}
			}
			// Try to match the identifier with Image name
			if r.MatchString(container.Image) {
				return container, nil
			}

		}
	}
	return dt.Container{}, ErrContainerNotFound
}

// DockerLogs returns a container's logs
func (a *DockerProductionAdapter) DockerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	a.climu.Lock()
	if a.cli == nil {
		if err := a.resetClient(); err != nil {
			return nil, err
		}
	}
	a.climu.Unlock()

	reader, err := a.cli.ContainerLogs(ctx, container, options)
	if err != nil {
		if !strings.Contains(err.Error(), "No such container") {
			a.climu.Lock()
			a.cli.Close()
			a.cli = nil
			a.climu.Unlock()
		}
		return nil, err
	}

	return reader, nil
}

// DockerEvents gets channels for consuming docker events subscription messages and errors
func (a *DockerProductionAdapter) DockerEvents(ctx context.Context, options types.EventsOptions) (
	<-chan events.Message, <-chan error, error,
) {
	a.climu.Lock()
	if a.cli == nil {
		if err := a.resetClient(); err != nil {
			return nil, nil, err
		}
	}
	a.climu.Unlock()

	msgchan, errchan := a.cli.Events(ctx, options)
	return msgchan, errchan, nil
}

// GetRunningContainers convenience wrapper to the default adapter for
// getting running containers.
func GetRunningContainers() ([]dt.Container, error) {
	return DefaultDockerAdapter.GetRunningContainers()
}

// MatchContainer convenience wrapper for finding containers using the
// default adapter.
func MatchContainer(containers []dt.Container, identifiers []string) (dt.Container, error) {
	return DefaultDockerAdapter.MatchContainer(containers, identifiers)
}

// DockerLogs convenience wrapper for reading container logs.
func DockerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	return DefaultDockerAdapter.DockerLogs(ctx, container, options)
}

// DockerEvents convenience wrapper for subscribing to docker events.
func DockerEvents(ctx context.Context, options types.EventsOptions) (
	<-chan events.Message, <-chan error, error,
) {
	return DefaultDockerAdapter.DockerEvents(ctx, options)
}

// GetEnvFromFile returns a map of environment variables parsed from a file.
func GetEnvFromFile(path string) (map[string]string, error) {
	return godotenv.Read(path)
}

// GetLogLine wraps a reader and returns the first line of text.
// Use to determine the validity of the log file.
func GetLogLine(scan *bufio.Scanner) ([]byte, error) {
	ok := scan.Scan()
	if !ok {
		err := scan.Err()
		if err != nil {
			return nil, scan.Err()
		}
		return nil, ErrEmptyLogFile
	}
	return scan.Bytes(), nil
}

func getDockerClient() (*client.Client, error) {
	defaultOpts := []client.Opt{
		client.FromEnv,
		client.WithAPIVersionNegotiation(),
	}

	if DefaultDockerHost != "" {
		defaultOpts = append(defaultOpts, client.WithHTTPClient(
			&http.Client{
				Transport: &http.Transport{
					Dial: func(network, addr string) (net.Conn, error) {
						return net.DialTimeout(network, addr, time.Second)
					},
				},
			}))
	}

	dockerCLI, err := client.NewClientWithOpts(defaultOpts...)
	if err != nil {
		return nil, err
	}

	return dockerCLI, nil
}

const (
	networkMainnet   = "mainnet"
	networkLocalnet  = "localnet"
	networkCanarynet = "canarynet"
	networkTestnet   = "testnet"
	networkBenchnet  = "benchnet"
)

// KnownNetworks list valid network strings
var KnownNetworks = []string{networkMainnet, networkLocalnet, networkTestnet, networkCanarynet, networkBenchnet}

// KnownNetwork returns true if s is in KnownNetworks
func KnownNetwork(s string) bool {
	s = strings.ToLower(s)

	for _, nw := range KnownNetworks {
		if strings.Contains(s, nw) {
			return true
		}
	}

	return false
}

// ExtractStringFromPEFEndpoint hits the given url extracts applies the regex
// to extract possible matches.
func ExtractStringFromPEFEndpoint(url string, re *regexp.Regexp) ([]string, error) {
	r, err := http.Get(url)
	if err != nil {
		return nil, err
	}

	if r.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("got response code: %v", r.StatusCode)
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return nil, err
	}

	if body == nil {
		return nil, fmt.Errorf("nil body")
	}

	if len(body) == 0 {
		return nil, fmt.Errorf("empty body")
	}

	matches := re.FindStringSubmatch(string(body))

	return matches, nil
}
