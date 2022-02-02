package discover

import (
	"agent/internal/pkg/global"
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"io/ioutil"
	"os"
	"os/exec"
	"regexp"
	"strconv"
	"strings"
	"text/template"
	"time"

	dt "github.com/docker/docker/api/types"
	docker "github.com/docker/docker/client"
	"github.com/joho/godotenv"
)

var (
	ErrContainerNotFound = errors.New("container not found")
	ErrEmptyLogFile      = errors.New("log file is empty")
)

func AutoConfig(reset bool) {
	if reset {
		ResetConfig()
	}

	switch global.Protocol {
	case "dapper":
		dapperDiscovery()
	case "algorand":
		algorandDiscovery()
	case "development":
		// developers configure their stuff themselves
		return
	default:
		panic(fmt.Sprintf("unknown protocol: %s", global.Protocol))
	}
}

// ResetConfig removes the protocol's configuration files.
// Allows discovery process to begin anew.
func ResetConfig() {
	var toRemove []string
	switch global.Protocol {
	case "dapper":
		toRemove = append(toRemove, global.DefaultDapperPath)
	case "algorand":
		toRemove = append(toRemove, global.DefaultAlgoPath)
	case "development":
		toRemove = append(toRemove, global.DefaultDapperPath, global.DefaultAlgoPath)
	}
	for _, item := range toRemove {
		if err := os.Remove(item); err != nil {
			if !errors.Is(err, fs.ErrNotExist) {
				fmt.Println("Reset failed to remove a config file")
			}
		}
	}
}

// GetRunningContainers returns a slice of all
// currently running Docker containers
func GetRunningContainers() ([]dt.Container, error) {
	cli, err := docker.NewClientWithOpts(docker.FromEnv, docker.WithAPIVersionNegotiation())
	if err != nil {
		return nil, err
	}

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

func generateConfig(templatePath, configPath string, config interface{}) error {
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		return err
	}

	configFile, err := os.Create(configPath)
	if err != nil {
		return err
	}

	return t.Execute(configFile, config)
}

// pidOf returns the PID of a specified process name.
// If process is not found an os.ExitError is returned.
// In case of abnormal output, strconv.NumError is returned.
func pidOf(name string) (int, error) {
	var err error
	var ret int
	// try pidof
	output, err := exec.Command("pidof", "-s", name).CombinedOutput()
	if err == nil {
		ret, err = strconv.Atoi(strings.Trim(string(output), "\n"))
		if err == nil {
			return ret, nil
		}
	}
	// try pgrep
	output, err = exec.Command("pgrep", "-n", name).Output()
	if err == nil {
		ret, err = strconv.Atoi(strings.Trim(string(output), "\n"))
		if err == nil {
			return ret, nil
		}
	}
	return 0, err
}

// pidArgs returns a string slice of the command line arguments
// of a specifid PID.
// First element is the executable path.
func pidArgs(pid int) ([]string, error) {
	pidStr := strconv.Itoa(pid)

	out, err := ioutil.ReadFile("/proc/" + pidStr + "/cmdline")
	if err != nil {
		return nil, err
	}
	args := bytes.Replace(out, []byte{0x0}, []byte{' '}, -1)
	return strings.Fields(string(args)), nil
}

// getEnvFromFile returns a map of environment variables parsed from a file.
func getEnvFromFile(path string) (map[string]string, error) {
	return godotenv.Read(path)
}

// getLogLine wraps a reader and returns the first line of text.
// Use to determine the validity of the log file.
func getLogLine(r io.Reader) ([]byte, error) {
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
