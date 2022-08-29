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

package discover

import (
	"errors"
	"io/fs"
	"os"

	"agent/internal/pkg/global"

	"go.uber.org/zap"
)

var (
	chain      global.Chain
	protocol   string
	configPath string
)

func AutoConfig(reset bool) global.Chain {
	log := zap.S()
	if reset {
		if err := chain.ResetConfig(); err != nil {
			log.Fatalw("failed to reset configuration", zap.Error(err))
		}
	}

	chn, ok := chain.(global.Chain)
	if !ok {
		log.Fatalw("blockchain package does not implement chain interface")
	}

	if ok := chain.IsConfigured(); ok {
		log.Info("protocol configuration OK")
		return chn
	}

	if _, err := chain.DiscoverContainer(); err != nil {
		log.Errorw("failed to automatically discover protocol configuration", zap.Error(err))
	}

	return chn
}

// ResetConfig removes the protocol's configuration files.
// Allows discovery process to begin anew.
func ResetConfig() {
	if err := os.Remove(configPath); err != nil {
		if !errors.Is(err, fs.ErrNotExist) {
			zap.S().Errorw("failed to remove a config file", zap.Error(err))
		}
	}
}
