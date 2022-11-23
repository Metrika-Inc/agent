# Metrika Agent
[![GitHub go.mod Go version of a Go module](https://img.shields.io/github/go-mod/go-version/Metrika-Inc/agent)](https://github.com/Metrika-Inc/agent) [![Linux](https://svgshare.com/i/Zhy.svg)](https://github.com/Metrika-Inc/agent) [![License](https://img.shields.io/badge/License-Apache_2.0-blue.svg)](https://github.com/Metrika-Inc/agent/blob/main/LICENSE)

[![CI Tests](https://github.com/Metrika-Inc/agent/actions/workflows/ci.yml/badge.svg?branch=main)](https://github.com/Metrika-Inc/agent/actions/workflows/ci.yml)

The Metrika Agent is a configurable software agent that regularly collects metrics and events from the host system as well as the blockchain node running on it. This data is then exported to one or more external sources via an [Exporter API](#exporter-api). By default the Metrika Agent sends data to the Metrika Platform for Blockchain Operational Intelligence. Find out more and [create an account for free](https://www.metrika.co).

## Installation
The Metrika Agent can be installed either as a systemd service or as a Docker container.

### Systemd
To install and run the agent as a systemd service you can download and run the [Installation Script](install.sh) using this one-liner:
```bash
MA_PLATFORM={platform_endpoint} MA_BLOCKCHAIN={blockchain} MA_API_KEY={api_key} bash -c "$(curl -L https://raw.githubusercontent.com/Metrika-Inc/agent/main/install.sh)"
```
The script serves as installer by default, but can also be used with flags `--upgrade` or `--uninstall`. In its default mode it does the following:
* Determines the latest version published on Github and downloads its binary for `{blockchain}`.
* Creates a the `metrikad` user and group on the system,
* Sets up Metrika Agent as a `systemd` service.
* Creates and populates `/etc/metrikad`:
  * [agent.yml](configs/agent.yml) - agent's main configuration file. API key inserted from environment variable `MA_API_KEY`.
  * `{blockchain}.yml` - blockchain node specific configuration file. Rendered from .template files under [configs](configs/).
* Creates and populates `/opt/metrikad` with the Metrika Agent binary.
* Starts Metrika Agent service.

The agent can run as a standalone binary, as long as configuration files are set up correctly.

### Systemd (non-root)
These instructions require running Caddyserver as Docker proxy. Before proceeding, make sure a proxy is listening on a `DOCKER_HOST` address and forwards requests to the host Docker daemon non-networked socket. You can find instructions about a recommended setup [here](#using-a-docker-reverse-proxy).

To install and run the agent as a systemd service without adding `metrikad` user to `docker` group, use the following one-liner:
```bash
MA_PLATFORM={platform_endpoint} DOCKER_HOST=tcp://127.0.0.1:2379 DOCKER_API_VERSION=1.41 MA_BLOCKCHAIN={blockchain} MA_API_KEY={api_key} bash -c "$(curl -sL https://raw.githubusercontent.com/Metrika-Inc/agent/main/install.sh)" -- --no-docker-grp
```

### Docker
To run the agent in a Docker container using the latest image use this one-liner:
```bash
docker run --rm \
    -d \
    -v /var/run/docker.sock:/var/run/docker.sock:ro \
    -v /proc:/host/proc:ro \
    -v /sys:/host/sys:ro \
    -e MA_PLATFORM={platform_endpoint} \
    -e MA_API_KEY={api_key} \
    --name metrikad-{blockchain} \
    --network host \
    ghcr.io/Metrika-Inc/agent:latest ./metrikad-{blockchain} -procfs /host/proc -sysfs /host/sys
```

### Docker (non-root)
Similarly to the corresponding systemd section, these instructions require setting up a docker proxy. You can find more about a recommended setup [here](#using-a-docker-reverse-proxy).

To run the agent in a Docker container with a user that has no elevated permissions use this one-liner:
```bash
docker run --rm \
    -d \
    -v /proc:/host/proc:ro \
    -v /sys:/host/sys:ro \
    -e MA_PLATFORM={platform_endpoint} \
    -e MA_API_KEY={api_key} \
    -e DOCKER_HOST=$DOCKER_HOST \
    -e DOCKER_API_VERSION=1.41 \
    --name metrikad-{blockchain} \
    --network host \
    --user metrikad \
    ghcr.io/Metrika-Inc/agent:latest ./metrikad-{blockchain} -procfs /host/proc -sysfs /host/sys
```

#### Docker image verification
Docker images are signed by Metrika using Github's [sigstore](https://sigstore.dev) [integration](https://github.blog/2021-12-06-safeguard-container-signing-capability-actions/). Images can be verified with [cosign](https://github.com/sigstore/cosign) following the steps below:
1. Install cosign by following these [instructions](https://docs.sigstore.dev/cosign/installation/).
2. Verify the image using Metrika's cosign public key:
```sh
cat <<EOT >> ma-cosign.pub
-----BEGIN PUBLIC KEY-----
MFkwEwYHKoZIzj0CAQYIKoZIzj0DAQcDQgAEP6OfMlstCuLkvrKVahxxvnaDW+Tw
wUJusBmsWSOMQVnYDb/jVfL/e/rgQxvFbulNiHGIRF3IXWzE3Jr8wTjdrg==
-----END PUBLIC KEY-----
EOT

cosign verify --key ma-cosign.pub ghcr.io/metrika-inc/agent:<tag>
```
You can read more details on verifying containers with cosign [here](https://github.com/sigstore/cosign#verify-a-container-against-a-public-key).

### Using a Docker reverse proxy
To avoid adding the metrikad user to the docker group, you can use a reverse proxy. The reverse proxy filters traffic before it reaches the non-networked Docker socket, preventing the agent process from speaking directly to it. The reverse proxy still needs a user that belongs to the docker group to run it so it can proxy requests to the UNIX socket.

The following process assumes a Debian based host and describes the steps required to install the agent, using [Caddy](https://caddyserver.com/) as a reverse proxy to the Docker daemon. The `Caddyfile` used is the least required configuration needed by the agent to perform its container discovery operations.

1. Install Caddy on your system by following these [instructions](https://caddyserver.com/docs/install).

2. Add user `caddy` to `docker` group:
```bash
usermod --append --groups docker caddy
```
3. Download the recommended [Caddyfile](https://raw.githubusercontent.com/Metrika-Inc/agent/main/caddy/Caddyfile) and replace `<ma_container>` with the name of the blockchain node container. Finally move `Caddyfile` to `/etc/caddy/Caddyfile`:
```bash
curl -sL https://raw.githubusercontent.com/Metrika-Inc/agent/main/caddy/Caddyfile | sed 's/<ma_container>/example-name/g' | tee /etc/caddy/Caddyfile
```
This `Caddyfile` filters requests to the Daemon based on the API paths needed by the agents.

4. Restart Caddy:
```bash
systemctl restart caddy
```
5. Use the following one-liner to install the Metrika Agent bypassing adding `metrikad` user to `docker` group and configuring `DOCKER_HOST` environment variable.
```bash
DOCKER_HOST=tcp://127.0.0.1:2379 DOCKER_API_VERSION=1.41 MA_BLOCKCHAIN={protocol} MA_API_KEY={api_key} bash -c "$(curl -sL https://raw.githubusercontent.com/Metrika-Inc/agent/main/install.sh) --no-docker-grp"
```
6. Test the proxy forwards requests to the Docker daemon for `metrikad` user:
```bash
sudo -u metrikad docker -H tcp://127.0.0.1:2379 ps
```
7. Test `metrikad` user cannot access the non-networked Docker Socket:
```bash
sudo -u metrikad docker ps
```
### Supported Blockchains
This is the list of all currently supported blockchains (several coming soon and we welcome contributions!):
* [flow](https://flow.com/)

## Configuration
The agent loads its configuration by looking at the following sources in order:
1. Load configuration `agent.yml` from `./configs` or current directory. The first to be found takes effect. If no files exist, proceed to step 2.
2. Load configuration from environment variables and overload any parameter set (except exporters).
3. Ensure sane defaults are set for configuration parameters that has not been set by a configuration file or an environment variable.
4. Ensure loaded configuration has all required parameters set.

Customization is possible by modifying the [agent.yml](configs/agent.yml) found in `/etc/metrikad/configs/agent.yml` after the installation. All configuration parameters (except `runtime.exporters`) can be overloaded by environment variables prefixed by `MA`. For example, to overload sampling frequency set `MA_RUNTIME_SAMPLING_INTERVAL=30s`.

### Parameter details
This covers a subset of configuration options that are most likely to be changed:
* `platform.api_key` - configured during install, an API key used for communicating with Metrika Platform. It maps the agent with your account.
* `buffer.max_heap_alloc` - maximum allowed allocations in heap (in bytes). Acts as a limit to prevent unlimited buffering. Default: `50MB`.
* `runtime.logging.outputs` - where Metrika outputs logs, can specify multiple sources. Default: `stdout` (journalctl). **Warning**: Metrika Agent does not take care of rotating the logs.
* `runtime.logging.level` - verbosity of the logs. Default - `info`. Recommended to increase to `debug` when troubleshooting.
* `runtime.exporters` **(work in progress)** - enable other exporters. More on this in [Exporter API](#exporter-api). Default: `false`.
* `runtime.watchers` - list of enabled watchers (collectors). More on this in [Watchers](#watchers).

## Agent internals
### Watchers
A watcher is responsible for collecting metrics or events from a single source at regular intervals. Watchers are composable - a watcher can collect data from another watcher to do additional transformations on data.

By default, Metrika Agent starts watchers for system metrics and a subset of blockchain-specific metrics and events.
### Exporter API

_Be advised that the Exporter API is work in progress and may change without notice._

All the data points collected by the enabled watchers are passed to one or more exporters, as defined by the `Exporter` interface in [exporter_registry.go](internal/pkg/global/exporter_registry.go).

By default, the only enabled exporter is Metrika Platform exporter, which encodes the data as protocol buffers ([proto definition](api/v1/proto/agent.proto)) and transmits them to Metrika Platform.

To disable Metrika Platform exporter, set the value of `platform.enabled` configuration parameter to `false`.

To enable any other exporter, specify its configuration under `runtime.exporters`. Example:
```yaml
runtime:
# ...
  exporters:
    file_stream_exporter:
      output_path: "/var/metrikad/agent.log"
```

More on exporter implementations can be found in [CONTRIBUTING.md](CONTRIBUTING.md#implementing-exporters)
## Troubleshooting

### Logs
Agent issues can be troubleshot with the help of logs. By default agent logs can be accessed via:
```bash
journalctl -u metrikad-{blockchain}.service
```

### Changing log level

_Requires_: `runtime.http_addr`.

You can modify the logging level without restarting the agent by sending an HTTP PUT request to the `/loglvl` endpoint:
```sh
curl -X PUT 127.0.0.1:9999/loglvl -d level=debug
```

### Prometheus Metrics

_Requires_: `runtime.http_addr` and `runtime.metrics_enabled`.

The agent listens for HTTP requests on the address specified by `runtime.metrics_addr`. You can scrape Prometheus metrics by hitting the `/metrics` endpoint, for example:

```sh
curl 127.0.0.1:9999/metrics # when runtime.http_addr=127.0.0.1:9999
```

### Blockchain Node Discovery Issues
The Metrika Agent attempts to discover a supported running blockchain node in the system. In case the blockchain node is containerized, Agent attempts to find the container by matching container name or image to a list of regular expressions specified in `/etc/metrikad/configs/{blockchain}.yml`. If the container name is not in the list of common names Metrika Agent is aware of, it can be added to the list of `containerRegex` in the aforementioned configuration file.

Please note that for containerized blockchain nodes, the agent needs to either:
1. Be added to the `docker` group OR
1. a suitable docker proxy needs to run on the host enabling partial access to the Docker API, and the `DOCKER_HOST` environment variable needs to be correctly set to point to said proxy, in order to allow the Metrika Agent to retrieve blockchain node log and configuration files.

### Other issues
For issues pertaining to the agent itself, feel free to open up an Issue here on Github and we will try and help you reach a resolution. If you are experiencing issues with the Metrika Platform please use [this form](https://metrika.atlassian.net/servicedesk/customer/portal/1/group/1/create/19).

## Contributing
See [CONTRIBUTING.md](CONTRIBUTING.md)

## Community
Reach out to us via [Discord](https://discord.gg/3tczKjK3ST)!

## License
Metrika Agent is licensed under the terms of [Apache 2.0 License](LICENSE).
