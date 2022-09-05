package collector

import (
	"fmt"
	"os"
	"testing"

	"agent/internal/pkg/global"
)

func TestMain(m *testing.M) {
	defaultConfigPathWas := global.ConfigFilePriority
	global.ConfigFilePriority = []string{"../../configs/agent.yml"}
	defer func() {
		global.ConfigFilePriority = defaultConfigPathWas
	}()

	if err := global.LoadAgentConfig(); err != nil {
		fmt.Printf("failed to load config: %v\n", err)

		os.Exit(1)
	}

	os.Exit(m.Run())
}
