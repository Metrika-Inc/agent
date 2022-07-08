package model

import (
	"time"

	structpb "google.golang.org/protobuf/types/known/structpb"
)

const (

	/* Additional event context is tracked by the following keys, depending on the event being generated:

	+----------------+--------+-------------------------------------------------------------------+
	| Event key name |  Type  |                            Description                            |
	+----------------+--------+-------------------------------------------------------------------+
	| uptime         | string | String formatted duration denoting how long the agent has been up |
	| endpoint       | string | A network address                                                 |
	| error          | string | An error string                                                   |
	| container_id   | string | The last discovered container ID                                  |
	| node_id        | string | The last discovered blockchain node ID                            |
	| node_type      | string | The last discovered blockchain node type                          |
	| node_version   | string | The last discovered blockchain node version                       |
	| offset_millis  | int64  | The agent's clock offset against NTP                              |
	| ntp_server     | string | The NTP server used by the agent's clock                          |
	+----------------+--------+-------------------------------------------------------------------+ */

	AgentUptimeKey   = "uptime"
	AgentProtocolKey = "protocol"
	EndpointKey      = "endpoint"
	ErrorKey         = "error"
	ContainerIDKey   = "container_id"
	NodeIDKey        = "node_id"
	NodeTypeKey      = "node_type"
	NodeVersionKey   = "node_version"
	OffsetMillisKey  = "offset_millis"
	NTPServerKey     = "ntp_server"

	/* core specific events */

	// AgentUpName The agent is up and running. Ctx: uptime
	AgentUpName = "agent.up"

	// AgentDownName The agent is dying
	AgentDownName = "agent.down"

	// AgentNetErrorName The agent failed to send data to the backend. Ctx: error
	AgentNetErrorName = "agent.net.error"

	// AgentHealthName The agent self-test results
	// TODO: not implemented
	AgentHealthName = "agent.health"

	/* chain specific events */

	// AgentNodeDownName The blockchain node is down. Ctx: container_id, node_id, node_type, node_version
	AgentNodeDownName = "agent.node.down"

	// AgentNodeUpName The blockchain node is up. Ctx: container_id, node_id, node_type, node_version
	AgentNodeUpName = "agent.node.up"

	// AgentNodeRestartName The blockchain node restarted. Ctx: container_id, node_id, node_type, node_version
	AgentNodeRestartName = "agent.node.restart"

	// AgentNodeLogMissingName The node log file has gone missing. Ctx: node_id, node_type, node_version
	AgentNodeLogMissingName = "agent.node.log.missing"

	// AgentNodeConfigMissingName The node configuration file has gone missing
	// TODO: not implemented
	AgentNodeConfigMissingName = "agent.node.config.missing"

	// AgentNodeLogFoundName The node log file has been found. Ctx: node_id, node_type,  node_version
	AgentNodeLogFoundName = "agent.node.log.found"

	// AgentConfigMissingName The agent configuration has gone missing
	// TODO: not implemented
	AgentConfigMissingName = "agent.config.missing"

	// AgentClockSyncName The agent synchronized its clock to NTP. Ctx: offset_millis, ntp_server
	AgentClockSyncName = "agent.clock.sync"

	// AgentClockNoSyncName The agent failed to synchronize its clock to NTP. Ctx: offset_millis, ntp_server, error
	AgentClockNoSyncName = "agent.clock.nosync"
)

// FromContext MUST be implemented by chain specific events
type FromContext interface {
	// New creates an event by keeping only a pre-configured
	// list of keys from a given map. The caller could further
	// modify the event values (i.e. sanitization) before
	// emitting. Time denotes the event's occurrence time.
	New(ctx map[string]interface{}, t time.Time) (*Event, error)
}

// NewWithFilteredCtx returns an event whose context is only
// a projection of the given keys. If no keys were given, the
// event's context will be empty.
func NewWithFilteredCtx(ctx map[string]interface{}, name string, t time.Time, keys ...string) (*Event, error) {
	if ctx == nil {
		return &Event{Name: name, Timestamp: t.UnixMilli()}, nil
	}

	filtered := make(map[string]interface{})
	for _, key := range keys {
		if val, ok := ctx[key]; ok {
			filtered[key] = val
		}
	}

	values, err := structpb.NewStruct(filtered)
	if err != nil {
		return nil, err
	}

	return &Event{Name: name, Timestamp: t.UnixMilli(), Values: values}, nil
}

// NewWithCtx returns an event whose context is equal to the given context.
func NewWithCtx(ctx map[string]interface{}, name string, t time.Time) (*Event, error) {
	keys := []string{}
	for key := range ctx {
		keys = append(keys, key)
	}

	ev, err := NewWithFilteredCtx(ctx, name, t, keys...)
	if err != nil {
		return nil, err
	}

	return ev, nil
}

func New(name string, t time.Time) (*Event, error) {
	return NewWithFilteredCtx(nil, name, t, []string{}...)
}
