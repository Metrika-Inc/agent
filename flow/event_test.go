package flow

import (
	"testing"
	"time"

	"agent/api/v1/model"

	"github.com/stretchr/testify/require"
	"google.golang.org/protobuf/types/known/structpb"
	"google.golang.org/protobuf/types/known/timestamppb"
)

func TestEvents(t *testing.T) {
	expOnFinalizedBlockStruct, _ := structpb.NewStruct(map[string]interface{}{
		"level":     "info",
		"node_role": "consensus",
		"node_id":   "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"hotstuff":  "telemetry",
		"chain":     "flow-mainnet",
		"path_id":   "2aad45bf-080d-4724-a40b-5af54fbc1dba",
		"view":      424144,
		"block_id":  "9cde727363d9cfb63267910b8937a5e8277e292649c912fa4ba6bb9e6facfdb6",
		"time":      "2022-02-16T18:58:03Z",
		"message":   "OnFinalizedBlock",
	})
	expOnProposingBlockStruct, _ := structpb.NewStruct(map[string]interface{}{
		"level":             "info",
		"node_role":         "consensus",
		"node_id":           "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"hotstuff":          "telemetry",
		"chain":             "flow-mainnet",
		"path_id":           "1f4a3a2b-6a2a-4377-9e53-faf977224f39",
		"view":              424070,
		"block_view":        424070,
		"block_id":          "a9db497cfc3bb12d120fa6921954fa813a23d90b7f1df040c78bdc3e633072b5",
		"block_proposer_id": "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"block_time":        "2022-02-16T18:56:06Z",
		"qc_block_view":     424069,
		"qc_block_id":       "c149d32e9967b74db586b9426cbd6fce7da1bd2b572ddd94d1608809fd7dad4a",
		"time":              "2022-02-16T18:56:06Z",
		"message":           "OnProposingBlock",
	})

	expOnOwnProposalStruct, _ := structpb.NewStruct(map[string]interface{}{
		"level":             "info",
		"node_role":         "consensus",
		"node_id":           "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"hotstuff":          "telemetry",
		"chain":             "flow-mainnet",
		"path_id":           "d2206793-034d-44e7-b2b6-9ff4d5d454ff",
		"view":              424145,
		"block_view":        424145,
		"block_id":          "4185ab208da8225c87837be8d9985f2fb65ba5545ad8ac83a8807f9e633ed799",
		"block_proposer_id": "e3650b2338dd3238e404fa6ee5d275501cd14d6dd94f1a56238a4e13d91dbb3a",
		"block_time":        "2022-02-16T18:58:04Z",
		"qc_view":           424144,
		"qc_block_id":       "4f4c5857564ecd68b5405bcf8e6a0b6d467151a4686c11f349554c7de21b91ca",
		"time":              "2022-02-16T18:58:05Z",
		"message":           "OnOwnProposal",
	})

	expOnReceiveProposalStruct, _ := structpb.NewStruct(map[string]interface{}{
		"level":             "info",
		"node_role":         "consensus",
		"node_id":           "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"hotstuff":          "telemetry",
		"chain":             "flow-mainnet",
		"path_id":           "d2206793-034d-44e7-b2b6-9ff4d5d454ff",
		"view":              424145,
		"block_view":        424145,
		"block_id":          "4185ab208da8225c87837be8d9985f2fb65ba5545ad8ac83a8807f9e633ed799",
		"block_proposer_id": "e3650b2338dd3238e404fa6ee5d275501cd14d6dd94f1a56238a4e13d91dbb3a",
		"block_time":        "2022-02-16T18:58:04Z",
		"qc_block_view":     424144,
		"qc_block_id":       "4f4c5857564ecd68b5405bcf8e6a0b6d467151a4686c11f349554c7de21b91ca",
		"time":              "2022-02-16T18:58:05Z",
		"message":           "OnReceiveProposal",
	})
	expOnVotingStruct, _ := structpb.NewStruct(map[string]interface{}{
		"level":            "info",
		"node_role":        "consensus",
		"node_id":          "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"hotstuff":         "telemetry",
		"chain":            "flow-mainnet",
		"path_id":          "a100ae5a-3141-438e-bda2-530961292521",
		"view":             424165,
		"voted_block_view": 424165,
		"voted_block_id":   "dfb17e8176f6d5f3ac0f9475ec23171e65a8f919da059e44868a8f6648460915",
		"voter_id":         "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"time":             "2022-02-16T18:58:34Z",
		"message":          "OnVoting",
	})
	expOnOwnVoteStruct, _ := structpb.NewStruct(map[string]interface{}{
		"level":            "info",
		"node_role":        "consensus",
		"node_id":          "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"hotstuff":         "telemetry",
		"chain":            "flow-mainnet",
		"path_id":          "a100ae5a-3141-438e-bda2-530961292521",
		"view":             424165,
		"voted_block_view": 424165,
		"voted_block_id":   "dfb17e8176f6d5f3ac0f9475ec23171e65a8f919da059e44868a8f6648460915",
		"time":             "2022-02-16T18:58:34Z",
		"message":          "OnOwnVote",
	})
	expOnBlockVoteReceivedFwdStruct, _ := structpb.NewStruct(map[string]interface{}{
		"level":      "info",
		"node_role":  "consensus",
		"node_id":    "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
		"compliance": "core",
		"block_view": 424089,
		"block_id":   "3ee140d3228fa4e2acd188e58503fe18ef50aa481057bf7b29169894d73b2c0e",
		"voter":      "14faeb5538c8791827163074cd5fdbdcf70c44648a98d835c63ebb386d5c4745",
		"vote_id":    "20ad3836a9c1bce7f45aa0a07ff209525b6dfb2942a21b87d54c0fe5308e59cb",
		"time":       "2022-02-16T18:56:42Z",
		"message":    "block vote received, forwarding block vote to hotstuff vote aggregator",
	})

	tests := []struct {
		evName string
		raw    map[string]interface{}
		exp    *model.Event
	}{
		{
			evName: "OnFinalizedBlock",
			raw: map[string]interface{}{
				"level":     "info",
				"node_role": "consensus",
				"node_id":   "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"hotstuff":  "telemetry",
				"chain":     "flow-mainnet",
				"path_id":   "2aad45bf-080d-4724-a40b-5af54fbc1dba",
				"view":      424144,
				"block_id":  "9cde727363d9cfb63267910b8937a5e8277e292649c912fa4ba6bb9e6facfdb6",
				"time":      "2022-02-16T18:58:03Z",
				"message":   "OnFinalizedBlock",
			},
			exp: &model.Event{
				Name:      "OnFinalizedBlock",
				Timestamp: timestamppb.Now().AsTime().UnixMilli(),
				Values:    expOnFinalizedBlockStruct,
			},
		},
		{
			evName: "OnProposingBlock",
			raw: map[string]interface{}{
				"level":             "info",
				"node_role":         "consensus",
				"node_id":           "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"hotstuff":          "telemetry",
				"chain":             "flow-mainnet",
				"path_id":           "1f4a3a2b-6a2a-4377-9e53-faf977224f39",
				"view":              424070,
				"block_view":        424070,
				"block_id":          "a9db497cfc3bb12d120fa6921954fa813a23d90b7f1df040c78bdc3e633072b5",
				"block_proposer_id": "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"block_time":        "2022-02-16T18:56:06Z",
				"qc_block_view":     424069,
				"qc_block_id":       "c149d32e9967b74db586b9426cbd6fce7da1bd2b572ddd94d1608809fd7dad4a",
				"time":              "2022-02-16T18:56:06Z",
				"message":           "OnProposingBlock",
			},
			exp: &model.Event{
				Name:      "OnProposingBlock",
				Timestamp: timestamppb.Now().AsTime().UnixMilli(),
				Values:    expOnProposingBlockStruct,
			},
		},
		{
			evName: "OnOwnProposal",
			raw: map[string]interface{}{
				"level":             "info",
				"node_role":         "consensus",
				"node_id":           "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"hotstuff":          "telemetry",
				"chain":             "flow-mainnet",
				"path_id":           "d2206793-034d-44e7-b2b6-9ff4d5d454ff",
				"view":              424145,
				"block_view":        424145,
				"block_id":          "4185ab208da8225c87837be8d9985f2fb65ba5545ad8ac83a8807f9e633ed799",
				"block_proposer_id": "e3650b2338dd3238e404fa6ee5d275501cd14d6dd94f1a56238a4e13d91dbb3a",
				"block_time":        "2022-02-16T18:58:04Z",
				"qc_view":           424144,
				"qc_block_id":       "4f4c5857564ecd68b5405bcf8e6a0b6d467151a4686c11f349554c7de21b91ca",
				"time":              "2022-02-16T18:58:05Z",
				"message":           "OnOwnProposal",
			},
			exp: &model.Event{
				Name:      "OnOwnProposal",
				Timestamp: timestamppb.Now().AsTime().UnixMilli(),
				Values:    expOnOwnProposalStruct,
			},
		},
		{
			evName: "OnReceiveProposal",
			raw: map[string]interface{}{
				"level":             "info",
				"node_role":         "consensus",
				"node_id":           "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"hotstuff":          "telemetry",
				"chain":             "flow-mainnet",
				"path_id":           "d2206793-034d-44e7-b2b6-9ff4d5d454ff",
				"view":              424145,
				"block_view":        424145,
				"block_id":          "4185ab208da8225c87837be8d9985f2fb65ba5545ad8ac83a8807f9e633ed799",
				"block_proposer_id": "e3650b2338dd3238e404fa6ee5d275501cd14d6dd94f1a56238a4e13d91dbb3a",
				"block_time":        "2022-02-16T18:58:04Z",
				"qc_block_view":     424144,
				"qc_block_id":       "4f4c5857564ecd68b5405bcf8e6a0b6d467151a4686c11f349554c7de21b91ca",
				"time":              "2022-02-16T18:58:05Z",
				"message":           "OnReceiveProposal",
			},
			exp: &model.Event{
				Name:      "OnReceiveProposal",
				Timestamp: timestamppb.Now().AsTime().UnixMilli(),
				Values:    expOnReceiveProposalStruct,
			},
		},
		{
			evName: "OnVoting",
			raw: map[string]interface{}{
				"level":            "info",
				"node_role":        "consensus",
				"node_id":          "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"hotstuff":         "telemetry",
				"chain":            "flow-mainnet",
				"path_id":          "a100ae5a-3141-438e-bda2-530961292521",
				"view":             424165,
				"voted_block_view": 424165,
				"voted_block_id":   "dfb17e8176f6d5f3ac0f9475ec23171e65a8f919da059e44868a8f6648460915",
				"voter_id":         "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"time":             "2022-02-16T18:58:34Z",
				"message":          "OnVoting",
			},
			exp: &model.Event{
				Name:      "OnVoting",
				Timestamp: timestamppb.Now().AsTime().UnixMilli(),
				Values:    expOnVotingStruct,
			},
		},
		{
			evName: "OnOwnVote",
			raw: map[string]interface{}{
				"level":            "info",
				"node_role":        "consensus",
				"node_id":          "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"hotstuff":         "telemetry",
				"chain":            "flow-mainnet",
				"path_id":          "a100ae5a-3141-438e-bda2-530961292521",
				"view":             424165,
				"voted_block_view": 424165,
				"voted_block_id":   "dfb17e8176f6d5f3ac0f9475ec23171e65a8f919da059e44868a8f6648460915",
				"time":             "2022-02-16T18:58:34Z",
				"message":          "OnOwnVote",
			},
			exp: &model.Event{
				Name:      "OnOwnVote",
				Timestamp: timestamppb.Now().AsTime().UnixMilli(),
				Values:    expOnOwnVoteStruct,
			},
		},
		{
			evName: "block vote received, forwarding block vote to hotstuff vote aggregator",
			raw: map[string]interface{}{
				"level":      "info",
				"node_role":  "consensus",
				"node_id":    "47a930e3ced158a62da7a36a7e8d8690a6987f54863a12ae37e8258fd7a6c410",
				"compliance": "core",
				"block_view": 424089,
				"block_id":   "3ee140d3228fa4e2acd188e58503fe18ef50aa481057bf7b29169894d73b2c0e",
				"voter":      "14faeb5538c8791827163074cd5fdbdcf70c44648a98d835c63ebb386d5c4745",
				"vote_id":    "20ad3836a9c1bce7f45aa0a07ff209525b6dfb2942a21b87d54c0fe5308e59cb",
				"time":       "2022-02-16T18:56:42Z",
				"message":    "block vote received, forwarding block vote to hotstuff vote aggregator",
			},
			exp: &model.Event{
				Name:      "BlockVoteReceived",
				Timestamp: timestamppb.Now().AsTime().UnixMilli(),
				Values:    expOnBlockVoteReceivedFwdStruct,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.evName, func(t *testing.T) {
			ev, ok := eventsFromContext[tt.evName]
			require.True(t, ok)

			gotEv, err := ev.New(tt.raw, time.Now())
			require.Nil(t, err)

			require.Equal(t, tt.exp.Name, gotEv.Name)
			require.Equal(t, tt.exp.Values.AsMap(), gotEv.Values.AsMap())
		})
	}
}
