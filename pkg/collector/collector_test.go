package collector

import (
	"fmt"
	"os"
	"testing"

	"agent/internal/pkg/global"
)

func TestMain(m *testing.M) {
	defaultConfigPathWas := global.DefaultAgentConfigPath
	global.DefaultAgentConfigPath = "../../configs/agent.yml"
	defer func() {
		global.DefaultAgentConfigPath = defaultConfigPathWas
	}()

	if err := global.LoadDefaultConfig(); err != nil {
		fmt.Printf("failed to load config: %v\n", err)

		os.Exit(1)
	}

	os.Exit(m.Run())
}
