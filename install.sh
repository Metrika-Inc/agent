#!/bin/bash

set -e
# set -x

# TODO: search for better logging practices
function goodbye {
  echo -e "${1}"
  retcode=1
  if [ -n "${2}" ]; then
    retcode=${2}
  fi
  exit $retcode
}

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



logo_gz="
H4sIAAAAAAAAA+1Xy2okMQy85ysW8gOWLVk2+pTMNcf5/+tWd7KBSDJ0ZjpsBsJADxSNrUepSn15
fSnXy+tLG1atbL/rn8ujY5VsWHcgyTCZxv36/PnlOm106xEfOy4OX59j0/TELLQbQqvN31PIVEzU
401tIO0R3u+mipAdPskaW/XH3BLx04/o+r0YecL0ab0Zh4Iy2SzWfUHPwqlreu/RXISMJmjqh6LJ
zo/Am7PSaZijGfmHKlYwmb6YzWNxihKs4yEYv0xXJOrKPr9J/WrV/H00eeAWrw80aLtaqsO17H3w
4aTZQPbM84fAK1FjDnwoCz6MPJ8lf0qqw8TNhI19PrUkIv8YrOpb+F7tp6EAQe1p8DZBEnBF8miS
HyvAmWkQamh8psVCYdBHzxFwlYe1oDFFUg8ipMxI3HMEo0CMC9zxqEUjEOggo6ZY6+c43G1catCF
kAPK0zCJYauApQ/jsG0o6AL2h8qBL/CJMOWVMHWUqAIKpBhgf75s0oyBP5A7Sk81aXk3VhQ6c7Ea
XYwUhEauXodogrw1BlgbaqbRtqhyfu8wjdqVNELRBY2+RLPtcYT1biVbpaTrGCQbkVefDieR3E+0
bVXEfyDDN28qK3zhfCvjDIuXSkqRb1+83mfE68vhxUvSWfpfXVhYwf1sy+yLSoOMw8KCDLwt0aHW
20qT+Roan8nG+9dKmMobxuljFME8gY74TFBOQcRBGBZ71RJfnLO49mjUmacsPuX+1TLgy9qP9MNj
7+2Mvc148BgL2C/2c7G/VYeXd5ISAAA=
"

logo=`echo "${logo_gz}" | base64 -d 2>/dev/null  | gzip -d 2>/dev/null`


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


# TODO: switch to prod
PLATFORM_ADDR="agent.sandboxes.aws.metrika.co:443"

###lib start##

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
  >&2 echo -e "\033[31;40m${1}${end}"
}

function download_binary {
    # TODO: implement me
    log_info "Downloading binary..."
}

function stop_service {
    echo "Stopping the agent, this might take few seconds..."

    $sudo_cmd systemctl stop -l "$BIN_NAME"|| true
    $sudo_cmd systemctl disable -l "$BIN_NAME" || true
}

function purge {
    stop_service

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

# Start installation
DISTRIBUTION=$(lsb_release -d 2>/dev/null | \
    grep -Eo "$KNOWN_DISTRIBUTION"  || \
    grep -Eo "$KNOWN_DISTRIBUTION" /etc/issue 2>/dev/null || \
    grep -Eo "$KNOWN_DISTRIBUTION" /etc/Eos-release 2>/dev/null || \
    grep -m1 -Eo "$KNOWN_DISTRIBUTION" /etc/os-release 2>/dev/null || \
    uname -s)
test -n "$DISTRIBUTION" || >&2 echo "Could not detect host OS distribution."

# Linux installation
if [ "$DISTRIBUTION" != "Darwin" ]; then
    echo "Detected host OS distribution: $DISTRIBUTION"

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
    goodbye "Distribution not supported: $DISTRIBUTION" 4
fi
