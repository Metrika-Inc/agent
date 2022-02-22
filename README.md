# Metrika Agent

## Wire Protocol

Wire Protocol model is used for communication between the agent and the platform it is pushing its data to. It is defined in [agent.proto](api/v1/proto/agent.proto) file.

The core piece of the protocol is `Message`. It contains:

1. `Timestamp (int64)` - encoded in milliseconds
1. `MessageType` - e.g. Event/Metric
1. `NodeState` - the state of the watched blockchain node
1. `AgentState` - the state of the agent
1. `Name (string)` - name of the item being sent
1. `Body (bytes)` - encoded payload of the message.

### Metric Encoding

We encode metrics in Prometheus Exposition Format (PEF). Useful links:
* [Specification](https://github.com/prometheus/docs/blob/main/content/docs/instrumenting/exposition_formats.md)
* [protobuf file](https://github.com/prometheus/client_model/blob/master/io/prometheus/client/metrics.proto)

### Event Encoding

TBD
