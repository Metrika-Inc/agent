package dapper

import (
	"agent/api/v1/model"
)

const (
	onFinalizedBlockName  = "OnFinalizedBlock"
	onProposingBlockName  = "OnProposingBlock"
	onReceiveProposalName = "OnReceiveProposal"
	onVotingName          = "OnVoting"

	onFinalizedBlockDesc  = "Block is finalized."
	onProposingBlockDesc  = "Validator of interest is proposing block."
	onReceiveProposalDesc = "Validator of interest is receiving proposal from other validators."
	onVotingDesc          = "Validator of interest is voting on block."
)

var (
	onFinalizedBlockKeys = []string{}
	onProposingBlockKeys = []string{"node_role", "node_id", "hotstuff", "chain", "path_id",
		"view", "block_view", "block_id", "block_proposer_id", "block_time", "qc_block_view",
		"qc_block_id", "time"}
	onReceiveProposalKeys = []string{"node_role", "node_id", "hotstuff", "chain", "path_id",
		"view", "block_view", "block_id", "block_proposer_id", "block_time", "qc_block_view",
		"qc_block_id", "time"}
	onVotingKeys = []string{"node_role", "node_id", "hotstuff", "chain", "path_id",
		"view", "voted_block_view", "voted_block_id", "voted_id", "time"}

	eventsFromContext = map[string]model.FromContext{
		onFinalizedBlockName:  onFinalizedBlock,
		onProposingBlockName:  onProposingBlock,
		onReceiveProposalName: onReceiveProposal,
		onVotingName:          onVoting,
	}

	onFinalizedBlock  = new(OnFinalizedBlock)
	onProposingBlock  = new(OnProposingBlock)
	onReceiveProposal = new(OnReceiveProposal)
	onVoting          = new(OnVoting)
)

/* OnFinalizedBlock*
{
	"level":"info",
	"node_role":"consensus",
	"node_id":"47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
	"hotstuff":"telemetry",
	"chain":"flow-mainnet",
	"path_id":"2aad45bf-080d-4724-a40b-5af54fbc1dba",
	"view":424144,
	"block_id":"9cde727363d9cfb63267910b8937a5e8277e292649c912fa4ba6bb9e6facfdb6",
	"time":"2022-02-16T18:58:03Z",
	"message":"OnFinalizedBlock"
}
*/

func isOnFinalizedBlock(v map[string]interface{}) bool {
	if val, ok := v["message"]; ok && val == "OnFinalizedBlock" {
		return true
	}

	return false
}

type OnFinalizedBlock struct{}

func (o *OnFinalizedBlock) New(v map[string]interface{}) (*model.Event, error) {
	if !isOnProposingBlock(v) {
		return nil, nil
	}

	ev, err := model.NewWithFilteredCtx(v, onFinalizedBlockName, onFinalizedBlockDesc, onFinalizedBlockKeys...)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

/* OnProposingBlock
{
	"level":"info",
	"node_role":"consensus",
	"node_id":"47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
	"hotstuff":"telemetry",
	"chain":"flow-mainnet",
	"path_id":"1f4a3a2b-6a2a-4377-9e53-faf977224f39",
	"view":424070,
	"block_view":424070,
	"block_id":"a9db497cfc3bb12d120fa6921954fa813a23d90b7f1df040c78bdc3e633072b5",
	"block_proposer_id":"47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
	"block_time":"2022-02-16T18:56:06Z",
	"qc_block_view":424069,
	"qc_block_id":"c149d32e9967b74db586b9426cbd6fce7da1bd2b572ddd94d1608809fd7dad4a",
	"time":"2022-02-16T18:56:06Z",
	"message":"OnProposingBlock"
}
*/

func isOnProposingBlock(v map[string]interface{}) bool {
	if val, ok := v["message"]; ok && val == "OnProposingBlock" {
		return true
	}

	return false
}

type OnProposingBlock struct{}

func (o *OnProposingBlock) New(v map[string]interface{}) (*model.Event, error) {
	if !isOnProposingBlock(v) {
		return nil, nil
	}

	ev, err := model.NewWithFilteredCtx(v, onProposingBlockName, onProposingBlockDesc, onProposingBlockKeys...)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

/* OnReceiveProposal*
{
	"level":"info",
	"node_role":"consensus",
	"node_id":"47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
	"hotstuff":"telemetry",
	"chain":"flow-mainnet",
	"path_id":"d2206793-034d-44e7-b2b6-9ff4d5d454ff",
	"view":424145,
	"block_view":424145,
	"block_id":"4185ab208da8225c87837be8d9985f2fb65ba5545ad8ac83a8807f9e633ed799",
	"block_proposer_id":"e3650b2338dd3238e404fa6ee5d275501cd14d6dd94f1a56238a4e13d91dbb3a",
	"block_time":"2022-02-16T18:58:04Z",
	"qc_block_view":424144,
	"qc_block_id":"4f4c5857564ecd68b5405bcf8e6a0b6d467151a4686c11f349554c7de21b91ca",
	"time":"2022-02-16T18:58:05Z",
	"message":"OnReceiveProposal"
}
*/

func isOnReceiveProposal(v map[string]interface{}) bool {
	if val, ok := v["message"]; ok && val == "OnReceiveProposal" {
		return true
	}

	return false
}

type OnReceiveProposal struct{}

func (o *OnReceiveProposal) New(v map[string]interface{}) (*model.Event, error) {
	if !isOnReceiveProposal(v) {
		return nil, nil
	}

	ev, err := model.NewWithFilteredCtx(v, onReceiveProposalName, onReceiveProposalDesc, onReceiveProposalKeys...)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

/* OnVoting*
{
	"level":"info",
	"node_role":"consensus",
	"node_id":"47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
	"hotstuff":"telemetry",
	"chain":"flow-mainnet",
	"path_id":"a100ae5a-3141-438e-bda2-530961292521",
	"view":424165,
	"voted_block_view":424165,
	"voted_block_id":"dfb17e8176f6d5f3ac0f9475ec23171e65a8f919da059e44868a8f6648460915",
	"voter_id":"47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
	"time":"2022-02-16T18:58:34Z",
	"message":"OnVoting"
}
*/

func isOnVoting(v map[string]interface{}) bool {
	if val, ok := v["message"]; ok && val == "OnVoting" {
		return true
	}

	return false
}

type OnVoting struct{}

func (o *OnVoting) New(v map[string]interface{}) (*model.Event, error) {
	if !isOnVoting(v) {
		return nil, nil
	}

	ev, err := model.NewWithFilteredCtx(v, onVotingName, onVotingDesc, onVotingKeys...)
	if err != nil {
		return nil, err
	}

	return ev, nil
}
