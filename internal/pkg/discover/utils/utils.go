package utils

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"io"
	"io/ioutil"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/docker/docker/api/types"
	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/events"
	docker "github.com/docker/docker/client"
	"github.com/joho/godotenv"
)

var (
	ErrContainerNotFound = errors.New("container not found")
	ErrEmptyLogFile      = errors.New("log file is empty")
)

// GetRunningContainers returns a slice of all
// currently running Docker containers
func GetRunningContainers() ([]dt.Container, error) {
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}
	defer cli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	containers, err := cli.ContainerList(ctx, dt.ContainerListOptions{})
	if err != nil {
		return nil, err
	}

	return containers, nil
}

// MatchContainer takes a slice of containers and regex strings.
// It returns the first running container to match any of the identifiers.
// If no matches are found, ErrContainerNotFound is returned.
func MatchContainer(containers []dt.Container, identifiers []string) (dt.Container, error) {
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

// PidOf returns the PID of a specified process name.
// If process is not found an os.ExitError is returned.
// In case of abnormal output, strconv.NumError is returned.
func PidOf(name string) (int, error) {
	var err error
	var ret int
	// try pidof
	output, err := exec.Command("pidof", "-s", name).Output()
	if err != nil {
		output, err = exec.Command("pgrep", "-n", name).Output()
		if err != nil {
			return 0, err
		}
	}

	ret, err = strconv.Atoi(strings.Trim(string(output), "\n"))
	if err != nil {
		return 0, err
	}
	return ret, nil
}

// PidArgs returns a string slice of the command line arguments
// of a specifid PID.
// First element is the executable path.
func PidArgs(pid int) ([]string, error) {
	pidStr := strconv.Itoa(pid)

	out, err := ioutil.ReadFile("/proc/" + pidStr + "/cmdline")
	if err != nil {
		return nil, err
	}
	args := bytes.Replace(out, []byte{0x0}, []byte{' '}, -1)
	return strings.Fields(string(args)), nil
}

// GetEnvFromFile returns a map of environment variables parsed from a file.
func GetEnvFromFile(path string) (map[string]string, error) {
	return godotenv.Read(path)
}

// GetLogLine wraps a reader and returns the first line of text.
// Use to determine the validity of the log file.
func GetLogLine(r io.Reader) ([]byte, error) {
	scan := bufio.NewScanner(r)
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

func DockerLogs(ctx context.Context, container string, options types.ContainerLogsOptions) (io.ReadCloser, error) {
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

	return cli.ContainerLogs(ctx, container, options)
}

func DockerEvents(ctx context.Context, options types.EventsOptions) (
	<-chan events.Message, <-chan error, error) {

	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, nil, err
	}

	msgchan, errchan := cli.Events(ctx, options)
	return msgchan, errchan, nil
}
