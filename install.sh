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
DNT=0

function goodbye {
  log_error "${1}"
  retcode=1
  if [ -n "${2}" ]; then
    retcode=${2}
  fi
  exit "$retcode"
}

logo_gz="
H4sIAAAAAAAAA+1XXUoEMQx+9wr7IniBJk2alhzFM+z9X/1mVLBpFmfZVVYQQSEMbfr9JZ5ey/n0
Wruzl+3n/Hz6SxUm796mEml3HS7t/PL1Qx7em7dY7XtVp+qlE3y43aFna45muM6nF3JTV5ur1bzj
gT1829wMLU7VQV7FeT7gyg6fHobYoxWauW/DW3UJeAn5KN7GnavULLntSOdKTgM6C2TrTrbdu/kK
6Y8oIiDFECId7/2RxUFLpeGXwiqr3zX6fXfZAhCzZd+Ct47TZ/dSp+1C5alqZYe4fAMxIYBcQ4YR
osBFAr0lpbdn3V+QQklSkKS6iguHlFmi9SHl0bZe54AdjndypEg2zWuoGt4I9GcboLgmNAEkl3vM
LDgf5MyEQ2zSvQbvF02CnvA4wRNnwqFhEhw8HYs3V4IWDkhjqNd24/i4VhQVvg0NA4MK14R5jMHY
XcKUNnAP1QZwQD5iOTiRCR6hxbVAwWC0+VzdUhGm/AYI4Eq8sNhcDDiuY4LjmCCDHvGuElqC+ji2
xBXIWJwMxJLd1t1imiwwGzC2GP406n53OxIkpSTrCuISnfLcvCy336CYbXfCXz7S4g9V08GST6Sw
nJgmjP/YcvIhbrna0HDbaoDfxTgN4Btks84IKhUJijkRrPq+PwYotzVgHR3gcrX1xxYewLvOAp/G
gYAULp/7Bl6KDoN10/3jQjU9Ib3sSJdrhqf/iHyiZYeQ7clevTM2ImMrsw+5pPxXfr3y9AZLEakK
2xAAAA==
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
  --reinstall             Reinstall/refresh the Metrika Agent installation.
  --uninstall             Stop and remove the Metrika Agent.
  --purge                 Stop, remove the Metrika Agent, including any agent configuration/data.
  --dnt                   Disable installer telemetry (telemetry helps Metrika improve the installer by sending anonymized errors!)
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
  echo -e "\033[1;33m${1}${end}"
}

function log_info {
  echo -e "\033[34m${1}${end}"
}

function log_ok {
  echo -e "\033[1;32m${1}${end}"
}

function log_error {
  >&2 echo -e "\033[1;31;40m${1}${end}"
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
    "--dnt")
      DNT=1
  esac
done


if [[ -z "${MA_BLOCKCHAIN}" ]]
then
    goodbye "MA_BLOCKCHAIN environment variable must be set to one of: '${SUPPORTED_BLOCKCHAINS[*]}'. Exiting."
fi

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


if [[ -z "${MA_API_KEY}" ]]
then
    goodbye "MA_API_KEY environment variable must be set before running the installation script. Exiting." 3
fi

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
if [ "$DISTRIBUTION" != "Darwin" ]; then
    log_info "Detected host OS distribution: $DISTRIBUTION"

    if service_exists; then
        printf "\nA previous installation of the agent was detected.\n"
        printf "\n1. Re-install the agent (will automatically uninstall first)."
        printf "\n2. Uninstall the agent (keep metadata) and exit."
        printf "\n3. Uninstall the agent completely."
        printf "\n4. Quit."
        printf "\n\n"
        echo -n "How would you like to proceed? [1-4q]+ "

        read -r ans
        if  [ "$ans" == "1" ]; then
          echo "Removing previous installation"
          uninstall
        elif  [ "$ans" == "2" ]; then
          echo "Removing previous installation"
          uninstall
          exit 0
        elif  [ "$ans" == "3" ]; then
          purge
          exit 0
        elif  [ "$ans" == "4" ] || [ "$ans" == "q" ]; then
          exit 0
        else
          goodbye "Uknown option $ans, aborting installation, goodbye" 10
        fi
    fi

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
