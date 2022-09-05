# Metrika Agent
[![CI Tests](https://github.com/Metrika-Inc/agent/actions/workflows/ci.yml/badge.svg?branch=v0%2Fmaster)](https://github.com/Metrika-Inc/agent/actions/workflows/ci.yml)

Metrika Agent is a configurable software that regularly collects metrics and events from host's machine and the blockchain node running on it. This data is then exported to one or more external sources via [Exporter API](#exporter-api), default being Metrika Platform.

## Installation
Metrika Agent is intended to be installed with a one-line command from the Metrika Platform that downloads and runs the [Installation Script](install.sh):
```bash
MA_BLOCKCHAIN={blockchain} MA_API_KEY={api_key} bash -c "$(curl -L https://raw.githubusercontent.com/Metrika-Inc/agent/v0/master/install.sh)"
```
The script serves as installer by default, but can also be used with flags `--upgrade` or `--uninstall`. In it's default mode it does the following:
* Downloads the latest stable release of Metrika Agent for `{blockchain}`
* Sets up Metrika Agent as `systemd` service
* Creates and populates `/etc/metrikad`:
  * [agent.yml](configs/agent.yml) - agent's main configuration file. API key inserted from environment variable `MA_API_KEY`
  * `{blockchain}.yml` - blockchain node specific configuration file. Rendered from .template files under [configs](configs/)
* Creates and populates `/opt/metrikad` with Metrika Agent binary
* Starts Metrika Agent

Metrika agent can run as a standalone binary, as long as configuration files are set up correctly.

### Supported Blockchains
This is the list of all currently supported blockchains (more coming soon):
* [flow](https://flow.com/)

## Configuration
The agent's configuration is set to sane defaults out of the box - in most use cases tinkering the configuration is unnecessary. 

Customization is possible by modifying the [agent.yml](configs/agent.yml) found in `/etc/metrikad/configs/agent.yml` after the installation.

### Parameter details
This covers a subset of configurations 
* `platform.api_key` - configured during install, an API key used for communicating with Metrika Platform. It maps the agent with your account.
* `buffer.max_heap_alloc` - maximum allowed allocations in heap (in bytes). Acts as a limit to prevent unlimited buffering. Default: `50MB`
* `runtime.logging.outputs` - where Metrika outputs logs, can specify multiple sources. Default: `stdout` (journalctl). **Warning**: Metrika Agent does not take care of rotating the logs.
* `runtime.logging.level` - verbosity of the logs. Default - `warning`. Recommended to increase to `debug` when troubleshooting.
* `runtime.use_exporters` **(work in progress)* - enable other exporters. More on this in [Exporter API](#exporter-api). Default: `false`.
* `runtime.watchers` - list of enabled watchers (collectors). More on this in [Watchers](#watchers)
## Agent internals
### Watchers
A watcher is responsible for collecting metrics or events from a single source at regular intervals. Watchers are composable - a watcher can collect data from another watcher to do additional transformations on data.

By default, Metrika Agent starts watchers for system metrics and a subset of blockchain-specific metrics and events.
### Exporter API
All the data points collected by the enabled watchers is passed to one or more exporters, as defined by the `Exporter` interface in [exporter_registry.go](internal/pkg/global/exporter_registry.go).

By default, the only enabled exporter is Metrika Platform exporter, which encodes the data as protocol buffers ([proto definition](api/v1/proto/agent.proto)) and transmits them to Metrika Platform. 
#### Implementing Exporters
Exporters can be implemented to encode the data in any desirable format and produce it anywhere.

`exporter.HandleMessage()` method will be called for every Message (metric or event) arriving from the Agent watchers. If the exporter implementation cannot keep up with the data flow, newly collected Messages will be dropped. As such, it's recommended to take implement buffering in the exporter implementation.

## Troubleshooting

### Logs
Agent issues can be troubleshot with the help of logs. By default agent logs can be accessed via
```bash
journalctl -u metrikad-{blockchain}.service
```

### Blockchain Node Discovery Issues
Metrika Agent makes its best effort to discover a running blockchain node in the system. In case the blockchain node is containerized, Agent attempts to find the container by matching container name or image to a list of regular expressions specified in `/etc/metrikad/configs/{blockchain}.yml`. If the container name is not in the list of common names Metrika Agent is aware of, it can be added to the list of `containerRegex` in the aforementioned configuration file.

### Other issues
Feel free to open up an Issue and we will try and help you reach a resolution.

## Community
Reach out to us via [Discord](https://discord.gg/3tczKjK3ST)!

## License
Metrika Agent is licensed under the terms of [Apache License](LICENSE).
