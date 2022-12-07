package flow

import (
	"os"
	"testing"

	"agent/internal/pkg/global"

	"github.com/stretchr/testify/require"
)

func TestNewFlowConfig(t *testing.T) {
	c := newFlowConfig("./testdata/flow-config.yml")
	conf, err := c.load()
	require.Nil(t, err)

	expConf := flowConfig{
		Client:         "flow-go",
		NodeID:         "088096a5a5e6caedbebc08a875cf081fb5dfd1baad7132f6fcc92dfdda1d5b89",
		ContainerRegex: []string{"flow-go"},
		EnvFilePath:    "/etc/flow/runtime-conf.env",
		PEFEndpoints: []global.PEFEndpoint{
			{
				URL: "http://127.0.0.1:3580/metrics",
				Filters: []string{
					"network_engine_messages_handled_total",
					"consensus_compliance_committed_epoch_final_view",
					"execution_block_data_uploader_block_data_upload_duration_ms",
					"collection_ingested_transactions_total",
					"verification_assigner_chunk_assigned_total",
					"access_transaction_submission_transaction_submission",
				},
			},
		},
	}
	require.Equal(t, expConf, conf)
}

func TestNewFlowConfig_OverloadEnv(t *testing.T) {
	os.Setenv("MA_FLOW_CONTAINER_REGEX", "flow-execution-go")
	os.Setenv("MA_FLOW_ENV_FILE", "foobar")

	c := newFlowConfig("./testdata/flow-config.yml")
	conf, err := c.load()
	require.Nil(t, err)

	expConf := flowConfig{
		Client:         "flow-go",
		NodeID:         "088096a5a5e6caedbebc08a875cf081fb5dfd1baad7132f6fcc92dfdda1d5b89",
		ContainerRegex: []string{"flow-execution-go"},
		EnvFilePath:    "foobar",
		PEFEndpoints: []global.PEFEndpoint{
			{
				URL: "http://127.0.0.1:3580/metrics",
				Filters: []string{
					"network_engine_messages_handled_total",
					"consensus_compliance_committed_epoch_final_view",
					"execution_block_data_uploader_block_data_upload_duration_ms",
					"collection_ingested_transactions_total",
					"verification_assigner_chunk_assigned_total",
					"access_transaction_submission_transaction_submission",
				},
			},
		},
	}
	require.Equal(t, expConf, conf)
}
