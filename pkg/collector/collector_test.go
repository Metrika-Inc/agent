package collector

import (
	"agent/internal/pkg/global"
	"os"
	"testing"

	"go.uber.org/zap"
)

func TestMain(m *testing.M) {
	defaultConfigPathWas := global.DefaultConfigPath
	global.DefaultConfigPath = "../../internal/pkg/global/agent.yml"
	defer func() {
		global.DefaultConfigPath = defaultConfigPathWas
	}()

	if err := global.LoadDefaultConfig(); err != nil {
		zap.L().Fatal("failed to load config", zap.Error(err))
	}

	os.Exit(m.Run())
}
