package collector

import (
	"fmt"
	"os"
	"testing"

	"agent/internal/pkg/global"
)

func TestMain(m *testing.M) {
	os.Setenv("MA_PLATFORM", "agent.test.metrika.co:443")
	os.Setenv("MA_API_KEY", "test_api_key")
	if err := global.LoadAgentConfig(); err != nil {
		fmt.Printf("failed to load config: %v\n", err)

		os.Exit(1)
	}

	os.Exit(m.Run())
}
