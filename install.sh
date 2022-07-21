#!/bin/bash

set -e
# set -x

PLATFORM_API_KEY=$MA_API_KEY
BLOCKCHAIN=$MA_BLOCKCHAIN
APP_NAME=metrikad
BIN_NAME=metrikad-$BLOCKCHAIN
MA_USER=metrikad
MA_GROUP=$MA_USER
APP_METADATA_DIR="/etc/$APP_NAME"
APP_INSTALL_DIR="/opt/$APP_NAME"
KNOWN_DISTRIBUTION="(Debian|Ubuntu)"
AGENT_CONFIG_NAME="agent.yml"
AGENT_DOWNLOAD_URL="http://0.0.0.0:8000/$BIN_NAME"
AGENT_CONFIG_DOWNLOAD_URL="http://0.0.0.0:8000/internal/pkg/global/$AGENT_CONFIG_NAME"
INSTALLER_VERSION="0.1"
SUPPORTED_BLOCKCHAINS=("dapper algorand")
SUPPORTED_ARCHS=("arm64 x86_64")
DNT=0

function goodbye {
  echo -e ""
  log_error "${1}"
  echo -e ""
  retcode=1
  if [ -n "${2}" ]; then
    retcode=${2}
  fi
  exit "$retcode"
}

logo_gz="
H4sIAAAAAAAAA1NQgIN8IIDRIAYXqhRYDMbgQpPKR1LDhWYeiKEAZXNhMQuhD9MsJPtgQvjciQG4
AMjuZGfgAAAA
"


<<<<<<< HEAD

if [ -z "$PS1" ]; then
  echo -e "\n\n${logo}\n"
fi

echo -e "METRIKA agent installation started for blockchain protocol node: '${MA_BLOCKCHAIN}'"


supported_blockchains=("flow algorand")
if [[ -z "${MA_BLOCKCHAIN}" ]]
then
    goodbye "MA_BLOCKCHAIN environment variable must be set to one of: '${supported_blockchains[*]}'. Exiting."
fi

case $MA_BLOCKCHAIN in
    flow)
        BLOCKCHAIN_CONFIG_TEMPLATE_NAME="flow.template"
        BLOCKCHAIN_CONFIG_NAME="flow.yml"
        ;;
    *)
        goodbye "Node type not supported: $MA_BLOCKCHAIN. The MA_BLOCKCHAIN envvar must be set to one of: '${supported_blockchains[*]}'. Exiting." 2
        ;;
esac

if [[ -z "${MA_API_KEY}" ]]
then
    goodbye "MA_API_KEY environment variable must be set before running the installation script. Exiting." 3
fi
=======
function usage {
  cat << EOF
usage: install.sh [options]
options:
  --upgrade               Upgrade the Metrika agent to the latest version.
  --reinstall             Reinstall/refresh the Metrika Agent installation.
  --uninstall             Stop and remove the Metrika Agent.
  --purge                 Stop, remove the Metrika Agent, including any agent configuration/data.
EOF
}
>>>>>>> b078aec (fixup: add usage, refactor install options.)


# TODO: switch to prod
PLATFORM_ADDR="agent.sandboxes.aws.metrika.co:443"

###lib start##

logo=$(echo "${logo_gz}" | base64 -d 2>/dev/null  | gzip -d 2>/dev/null)
function print_header {
  if [ -z "$PS1" ]; then
    echo -e "\n\n${logo}\n"
    cat << EOF
------------------------------
Metrika Agent Installer ${INSTALLER_VERSION}
------------------------------
EOF
  fi
}


if [ "$UID" = "0" ]; then
    sudo_cmd=''
else
    sudo_cmd='sudo'
fi

end="\033[0m"

function log_warn {
  echo -e "\033[1;33m ⚠️  ${1}${end}"
}

function log_info {
  echo -e "\033[34m 🛈  ${1}${end}"
}

function log_ok {
  echo -e "\033[1;32m 🟢  ${1}${end}"
}

function log_error {
  >&2 echo -e "\033[1;31;40m ⛔  ${1}${end}"
}

function download_binary {
    # TODO: implement me
    log_info "Downloading binary..."
}

function stop_service {
    log_info "Stopping the agent, this might take few seconds..."

    $sudo_cmd systemctl stop -l "$BIN_NAME"|| true
    $sudo_cmd systemctl disable -l "$BIN_NAME" || true
}

function send_telemetry_event {
  # TODO(cosmix): implement me!
  true
}

function sanity_check {

  # CURL 
  test -x "$(which curl)" || goodbye "curl is missing from your system. Please install it and try again."
  arch=$(uname -i)

  # SYSTEM ARCHITECTURE
  case $arch in
    "x86_64")
      true
      ;;
    "arm64")
      true
      ;;
    "*")
      goodbye "Unsupported architecture. Metrika Agent currently supports: '${SUPPORTED_ARCHS[*]}'"
  esac

  # MA_BLOCKCHAIN envvar
  if [[ -z "${MA_BLOCKCHAIN}" ]]
  then
    goodbye "MA_BLOCKCHAIN environment variable must be set to one of: '${SUPPORTED_BLOCKCHAINS[*]}'. Exiting."
  fi

  # MA_API_KEY envvar
  if [[ -z "${MA_API_KEY}" ]]
  then
    goodbye "MA_API_KEY environment variable must be set before running the installation script. Exiting." 3
  fi

  if [ "$DISTRIBUTION" == "Darwin" ]; then
    goodbye "Metrika Agent does not support macOS just yet."
  fi

  if [[ "$DISTRIBUTION" =~ .*BSD ]]; then
    goodbye "Metrika Agent does not support BSD just yet."
  fi
}

function purge {
    stop_service

    if [[ -z $APP_METADATA_DIR ]];
    then
      # remove metadata
      $sudo_cmd rm -f $APP_METADATA_DIR/configs/*
      $sudo_cmd rm -f $APP_METADATA_DIR/$AGENT_CONFIG_NAME
      $sudo_cmd rmdir $APP_METADATA_DIR/configs || true
      $sudo_cmd rmdir $APP_METADATA_DIR || true

      # remove installation directory
      $sudo_cmd rm -f $APP_INSTALL_DIR/"$BIN_NAME"
      $sudo_cmd rmdir --ignore-fail-on-non-empty $APP_INSTALL_DIR || true

      # remove user artifacts
      $sudo_cmd userdel $MA_USER || true
    fi
}

function uninstall {
    stop_service

    $sudo_cmd rm -f "/lib/systemd/system/$BIN_NAME.service"
    $sudo_cmd rm -f "$APP_INSTALL_DIR/$BIN_NAME"
    $sudo_cmd userdel $MA_USER || true
}

function service_exists {
    status=$("$sudo_cmd" systemctl --no-pager list-units --full -all | grep -F "$BIN_NAME".service)

    if [[ -n "$status" || -f "$APP_INSTALL_DIR/$BIN_NAME" ]]; then
        return 0
    fi

    return 1
}

function create_systemd_service {
    log_info "Creating systemd service..."
    service=$(envsubst <<EOF
[Unit]
Description=Metrika Agent ($BLOCKCHAIN)
After=network.target
StartLimitIntervalSec=0

[Service]
Restart=always
RestartSec=5
User=$MA_USER
ExecStart=/usr/bin/env $APP_INSTALL_DIR/$BIN_NAME

[Install]
WantedBy=multi-user.target
EOF
)
    log_info "\033[1;32m $service ${end}"
    echo "$service" | $sudo_cmd tee "/lib/systemd/system/$BIN_NAME.service"
    cd /etc/systemd/system
    $sudo_cmd systemctl daemon-reload
    $sudo_cmd systemctl enable "$BIN_NAME.service"
    $sudo_cmd systemctl --no-pager start -l "$BIN_NAME.service"
    $sudo_cmd systemctl --no-pager status -l "$BIN_NAME.service"
}

###lib end##

# ----------------------------------
# -- Mainline program starts here --
# ----------------------------------


print_header

for arg in "$@"; do
  case "$arg" in
    "--reinstall")
      uninstall
      ;;
    "--uninstall")
      uninstall
      exit 0
      ;;
    "--purge")
      purge
      exit 0
      ;;
    "--help")
      usage
      exit 0
      ;;
  esac
done

sanity_check



case $MA_BLOCKCHAIN in
    dapper)
        BLOCKCHAIN_CONFIG_TEMPLATE_NAME="dapper.template"
        BLOCKCHAIN_CONFIG_NAME="dapper.yml"
        ;;
    algorand)
        BLOCKCHAIN_CONFIG_TEMPLATE_NAME="algorand.template"
        BLOCKCHAIN_CONFIG_NAME="algorand.yml"
        ;;
    *)
        goodbye "Node type not currently supported: $MA_BLOCKCHAIN. The MA_BLOCKCHAIN envvar must be set to one of: '${SUPPORTED_BLOCKCHAINS[*]}'.\nPlease find us on Discord or email us at support@metrika.co if you want to see $MA_BLOCKCHAIN supported. Exiting." 2
        ;;
esac


 log_info "METRIKA agent installation started for blockchain protocol node: '${MA_BLOCKCHAIN}'"

# Start installation
DISTRIBUTION=$(lsb_release -d 2>/dev/null | \
    grep -Eo "$KNOWN_DISTRIBUTION"  || \
    grep -Eo "$KNOWN_DISTRIBUTION" /etc/issue 2>/dev/null || \
    grep -Eo "$KNOWN_DISTRIBUTION" /etc/Eos-release 2>/dev/null || \
    grep -m1 -Eo "$KNOWN_DISTRIBUTION" /etc/os-release 2>/dev/null || \
    uname -s)
test -n "$DISTRIBUTION" || >&2 log_warn "Could not detect host OS distribution."



# Linux installation

    echo "Creating system group(user): $MA_GROUP($MA_USER)"
    getent passwd "$MA_USER" >/dev/null || \
        $sudo_cmd adduser --system --group --home $APP_INSTALL_DIR --shell /sbin/nologin "$MA_USER" && \
        { $sudo_cmd usermod -L "$MA_USER" || \
            log_warn "Cannot lock the 'metrika-agent' user account"; }
    $sudo_cmd usermod -aG docker $MA_USER

    echo "Preparing agent metadata directory: $APP_METADATA_DIR"
    $sudo_cmd mkdir -p $APP_METADATA_DIR/configs
    $sudo_cmd chown -R $MA_GROUP:$MA_USER $APP_METADATA_DIR

    # TODO download_binary
    echo "Downloading agent binary"
    $sudo_cmd mkdir -p $APP_INSTALL_DIR
    # wget --quiet -O "$BIN_NAME" "$AGENT_DOWNLOAD_URL"
    # wget --quiet -O "$AGENT_CONFIG_NAME" "$AGENT_CONFIG_DOWNLOAD_URL"

    echo "Installing agent..."
    $sudo_cmd chown -R $MA_GROUP:$MA_USER $APP_INSTALL_DIR
    $sudo_cmd cp -t $APP_INSTALL_DIR "$BIN_NAME"
    $sudo_cmd cp -t $APP_METADATA_DIR/configs configs/$AGENT_CONFIG_NAME
    $sudo_cmd sed -i "s/<api_key>/$PLATFORM_API_KEY/g" $APP_METADATA_DIR/configs/$AGENT_CONFIG_NAME
    $sudo_cmd sed -i "s/<platform_addr>/$PLATFORM_ADDR/g" $APP_METADATA_DIR/configs/$AGENT_CONFIG_NAME
    $sudo_cmd cp -t $APP_METADATA_DIR/configs configs/$BLOCKCHAIN_CONFIG_TEMPLATE_NAME

    create_systemd_service

    # TODO
    # enable the agent systemd service
    # finally start the service
else
    # macOS
    goodbye "Operating System currently not supported: $DISTRIBUTION" 4
fi
