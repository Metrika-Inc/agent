package model

import (
	"time"

	structpb "google.golang.org/protobuf/types/known/structpb"
)

const (
	/* core specific events */

	// AgentUpName The agent is up and running
	// Ctx: uptime, string formatted duration denoting how long the agent has been up
	AgentUpName = "agent.up"

	// AgentDownName The agent is dying
	// Ctx: signal_number
	AgentDownName = "agent.down"

	// AgentNetErrorName The agent failed to send data to the backend
	// Ctx: endpoint, error
	AgentNetErrorName = "agent.net.error"

	// TODO: not currently implemented
	// AgentHealthName The agent self-test results
	// Ctx: state: enum(HEALTHY, UNHEALTHY), errors
	AgentHealthName = "agent.health"

	/* chain specific events */

	// AgentNodeDownName The blockchain node is down
	// Ctx: old_pid
	AgentNodeDownName = "agent.node.down"

	// AgentNodeUpName The blockchain node is up
	// Ctx: pid
	AgentNodeUpName = "agent.node.up"
	AgentNodeUpDesc = "The node is up"
	// pid

	AgentNodeRestartName = "agent.node.restart"
	AgentNodeRestartDesc = "The node restarted"
	// old_pid, new_pid

	AgentNodeLogMissingName = "agent.node.log.missing"
	AgentNodeLogMissingDesc = "The node log file has gone missing"
	// docker regex || path

	AgentNodeConfigMissingName = "agent.node.config.missing"
	AgentNodeConfigMissingDesc = "The node configuration file has gone missing"
	// path

	AgentNodeLogFoundName = "agent.node.log.found"
	AgentNodeLogFoundDesc = "The node log file has been found"

	AgentConfigMissingName = "agent.config.missing"
	AgentConfigMissingDesc = "The agent configuration has gone missing"
	// path

	AgentClockSyncName = "agent.clock.sync"
	AgentClockSyncDesc = "The agent synchronized its clock to NTP"
	// offset_millis, ntp_server

	AgentClockNoSyncName = "agent.clock.nosync"
	AgentClockNoSyncDesc = "The agent failed to synchronize its clock to NTP"
	// ntp_server, error
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

	return &Event{Name: name, Values: values}, nil
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
