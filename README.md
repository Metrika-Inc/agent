# Metrika Agent

Metrika Agent is built to collect general system metrics, as well as a protocol-specific metrics and events.

## Agent Binaries

A single Agent binary supports only one protocol/node at a time. This is achieved with go build tags. Every protocol implements the `global.Chain` interface.

### Implementing support for new Protocol

* Create a new package in the project root (existing examples: [dapper](dapper/), [algorand](algorand/))
* Create a struct to implement `global.Chain` interface
* Add the package name to the supported protocol list in [Makefile](Makefile)
* Compile with `make build-[package-name]`

### global.Chain

Found in [global](internal/pkg/global/agent.go) package.

## Wire Protocol

The Metrika team intends to use OpenTelemetry (OTLP) once the Logs and Metrics specifications are stable and SDKs are available for Golang. Currently we use a custom wire Protocol model for communication between the agent and the platform it is pushing its data to. It is defined in [agent.proto](api/v1/proto/agent.proto) file.

The core piece of the protocol is `Message`. It contains:

1. `Timestamp (int64)` - encoded in milliseconds.
1. `MessageType (enum)` - e.g. Event/Metric.
1. `Name (string)` - name of the item being sent.
1. `Body (bytes)` - encoded payload of the message.

### Metric Encoding

We encode metrics with OpenMetrics model. It is an open-source metrics specification which extends Prometheus Exposition Format (PEF). Useful links:
* [Specification](https://github.com/OpenObservability/OpenMetrics/blob/main/specification/OpenMetrics.md)
* [protobuf file](https://github.com/OpenObservability/OpenMetrics/blob/main/proto/openmetrics_data_model.proto)

### Event Encoding

Event encoding is defined in the same [agent.proto](api/v1/proto/agent.proto) file under the type `Event`:
1. `Name (string)` - event name, identifier.
1. `Desc (string)` - short description of the event.
1. `Values (google.protobuf.Struct)` - holds 0 or more key/value pairs, provides context.

Useful links:
* [google.protobuf.Struct](https://developers.google.com/protocol-buffers/docs/reference/google.protobuf#google.protobuf.Struct)
