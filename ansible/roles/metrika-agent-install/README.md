## Description
This ansible role handles the installation of the Metrika agent on linux operating systems. Some configurations are not configurable and thus are hard coded to ensure that the agent works as expected. This role expects to have root permissions to download the agent configuration and create the appropriate users and groups

## Variables
When using this role, the following variables are expected
* `agent_version` - Version of the agent to be downloaded
* `arch` - Agent architecture (arm64 or amd64)
* `installation_path` - Installation path that must be writable
* `metrika_agent_api_key` - API key provided by Metrika

## Example Usage in Playbook
```
- hosts: 1.1.1.1
  become: yes
  roles:
    - role: metrika-agent-install
      vars:
        agent_version: "v0.2.1"
        arch: "amd64"
        installation_path: "/opt/"
```

## Example Ansible Playbook Command
```
ansible-playbook my-playbook.yml -e "metrika_agent_api_key=mysecret-key"
```