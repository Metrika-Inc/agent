# Contributing to Metrika Agent

Thank you for contributing or considering to contribute to the Metrika Agent! This document will outline the general know-how to contribute to Metrika Agent.

## General guidelines
Please be respectful to the developers and fellow contributors in any channel of communication.

## Opening Issues
Preferred way to report an issue is via GitHub [issue tracker](https://github.com/Metrika-Inc/agent/issues/). Before opening a new issue, try and see if it hasn't been reported already.

Guidelines reporting a bug or issue:

* Include the entire log file or a relevant fragment if applicable. By default logs are saved in `journalctl`.
```bash
journalctl -u metrikad-{protocol}.service -S yesterday > agent.log
```

* Specify the protocol you're running the Metrika Agent with e.g. `flow`
* Specify the agent version
```bash
sudo /opt/metrikad/metrikad-{protocol} --version
```
* Include any other details you believe may be relevant.

## Getting started with code contributions

### Development environment
To work on an agent development you will need:
* GNU `make`
* Go `1.18` or later

To ensure everything is set up correctly, simply run `make build`, which should build every metrika agent binary version for your architecture.

### Pull Requests
If you'd like to contribute to the Metrika Agent, you can do so by forking it and submit a Pull Request.

Preparing a Pull Request:
* Have a link to an issue (if it's solving an existing one).
* Have a clear name and description for the Pull Request.
* Ensure the code is formatted with `gofmt`.
* Try sticking with established Go naming conventions as per [Effective Go](https://go.dev/doc/effective_go).

When the pull request is open:
* We may ask for changes to be made before a PR can be merged, either using suggested changes or pull request comments. You can apply suggested changes directly through the UI. You can make any other changes in your fork, then commit them to your branch.
* As you update your PR and apply changes, mark each conversation as resolved. 

## Contribution Tips
This section includes suggestions for contributing to specific parts of the Metrika Agent's codebase.

### Implementing Exporters
Exporters can be implemented to encode the data in any desirable format and produce it anywhere.

`exporter.HandleMessage()` method will be called for every Message (Metric or Event) arriving from the Agent watchers. If the exporter implementation cannot keep up with the data flow, newly collected Messages will eventually be dropped. As such, it's recommended to implement buffering in the exporter implementation.
