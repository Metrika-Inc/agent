# Metrika Agent
[![CI Tests](https://github.com/Metrika-Inc/agent/actions/workflows/ci.yml/badge.svg?branch=v0%2Fmaster)](https://github.com/Metrika-Inc/agent/actions/workflows/ci.yml)

Metrika Agent is a configurable software that regularly collects metrics and events from host's machine and the blockchain node running on it. This data is then exported to one or more external sources via [Exporter API](#exporter-api), default being Metrika Platform.

## Installation
Metrika Agent is intended to be installed with a one-line command from the Metrika Platform that downloads and runs the [Installation Script](install.sh):
```bash
MA_BLOCKCHAIN={blockchain} MA_API_KEY={api_key} bash -c "$(curl -L https://raw.githubusercontent.com/Metrika-Inc/agent/v0/master/install.sh)"
```
The script:
* Serves as installer, updater, and uninstaller
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


## Agent internals
```
Concept of watcher
Exporter API
```
### Exporter API

## Troubleshooting
```
journalctl
changing log levels/output
```
## Community
Reach out to us via [Discord](https://discord.gg/3tczKjK3ST)!

## License
Metrika Agent is licensed under the terms of [Apache License](LICENSE).
>>>>>>> 8e6ce38 ([WIP]: Update README.md)
