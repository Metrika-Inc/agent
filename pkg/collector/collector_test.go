package collector

import (
	"agent/internal/pkg/global"
	"os"
	"testing"

	"github.com/sirupsen/logrus"
)

func TestMain(m *testing.M) {
	defaultConfigPathWas := global.DefaultConfigPath
	global.DefaultConfigPath = "../../internal/pkg/global/agent.yml"
	defer func() {
		global.DefaultConfigPath = defaultConfigPathWas
	}()

	if err := global.LoadDefaultConfig(); err != nil {
		logrus.Fatal(err)
	}

	os.Exit(m.Run())
}
