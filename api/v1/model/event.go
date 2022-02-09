package model

import (
	"encoding/json"
	"fmt"
)

const (
	/* core specific events */

	AgentUpName = "agent.up"
	AgentUpDesc = "The agent is up and running"
	// system_uptime

	AgentDownName = "agent.down"
	AgentDownDesc = "The agent is dying"
	// signal_number

	AgentNetErrorName = "agent.net.error"
	AgentNetErrorDesc = "The agent failed to send data to the backend"
	// endpoint, error

	AgentHealthName = "agent.health"
	AgentHealthDesc = "The agent self-test results"
	// state: enum(HEALTHY, UNHEALTHY), errors

	AgentNodeDownName = "agent.node.down"
	AgentNodeDownDesc = "The node is down"
	// old_pid

	AgentNodeUpName = "agent.node.up"
	AgentNodeUpDesc = "The node is up"
	// pid

	/* chain specific events */

	AgentNodeRestartName = "agent.node.restart"
	AgentNodeRestartDesc = "The node restarted"
	// old_pid, new_pid

	AgentNodeLogMissingName = "agent.node.log.missing"
	AgentNodeLogMissingDesc = "The node log file has gone missing"
	// path

	AgentNodeConfigMissingName = "agent.node.config.missing"
	AgentNodeConfigMissingDesc = "The node configuration file has gone missing"
	// path

	AgentNodeLogFoundName = "agent.node.log.found"
	AgentNodeLogFoundDesc = "The node log file has been found"

	AgentConfigMissingName = "agent.config.missing"
	AgentConfigMissingDesc = "The agent configuration has gone missing"
	// path

)

// FromContext MUST be implemented by chain specific events
type FromContext interface {

	// New creates an event by keeping only a pre-configured
	// list of keys from the map. The caller could further
	// modify the event values (i.e. sanitization) before
	// emitting.
	New(ctx map[string]interface{}) (*Event, error)
}

// NewWithFilteredCtx returns an event whose context is only
// a projection of the given keys. If no keys were given, the
// event's context will be empty.
func NewWithFilteredCtx(ctx map[string]interface{}, name, desc string, keys ...string) (*Event, error) {
	if ctx == nil {
		return &Event{Name: name, Desc: desc}, nil
	}

	values := make(map[string]interface{})

	for _, key := range keys {
		val, ok := ctx[key]
		if !ok {
			return nil, fmt.Errorf("%s, missing key %q", name, key)
		}
		values[key] = val
	}

	valuesb, err := json.Marshal(values)
	if err != nil {
		return nil, err
	}

	return &Event{Name: name, Desc: desc, Values: valuesb}, nil
}

// NewWithCtx returns an event whose context is equal to the given context.
func NewWithCtx(ctx map[string]interface{}, name, desc string) *Event {
	keys := []string{}
	for key := range ctx {
		keys = append(keys, key)
	}

	ev, _ := NewWithFilteredCtx(ctx, name, desc, keys...)

	return ev
}

func New(name, desc string) *Event {
	ev, _ := NewWithFilteredCtx(nil, name, desc, []string{}...)

	return ev
}
