package discover

import (
	"agent/internal/pkg/global"
	"context"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"regexp"
	"text/template"
	"time"

	dt "github.com/docker/docker/api/types"
	"github.com/docker/docker/client"
)

var ErrContainerNotFound = errors.New("container not found")

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

type ValidatorState int

const (
	Unchecked ValidatorState = iota
	Empty
	Invalid
	Valid
)

type ItemExecutor struct {
	Name         string
	ValidateFunc func() ValidatorState
	DiscoverFunc func() error
}

// GetContainer takes a slice of regex strings and returns
// the first running container to match any of the identifiers.
// If no matches are found, ErrContainerNotFound is returned.
func GetContainer(identifiers []string) (dt.Container, error) {
	ctx, _ := context.WithTimeout(context.Background(), 5*time.Second)
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return dt.Container{}, err
	}
	containers, err := cli.ContainerList(ctx, dt.ContainerListOptions{})
	if err != nil {
		return dt.Container{}, err
	}

	for _, container := range containers {
		for _, rStr := range identifiers {
			r := regexp.MustCompile(rStr)

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

func generateConfig(templatePath, configPath string, config interface{}) {
	t, err := template.ParseFiles(templatePath)
	if err != nil {
		panic(err)
	}

	configFile, err := os.Create(configPath)
	if err != nil {
		panic(err)
	}

	// t.ExecuteTemplate(configFile, "configs/dapper.template", global.DapperConf)
	t.Execute(configFile, config)

}
