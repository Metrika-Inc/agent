#!/bin/bash

set -e
set -x

# TODO: search for better logging practices
function goodbye {
  echo -e "${1}"
  exit 1
}

supported_blockchains=("dapper algorand")
if [[ -z "${MA_BLOCKCHAIN}" ]]
then
    echo -e "MA_BLOCKCHAIN environment variable must be set to one of: '${supported_blockchains[*]}'. Exiting."

    exit 1
fi

case $MA_BLOCKCHAIN in
    dapper)
        echo -e "dapper"
        BLOCKCHAIN_CONFIG_TEMPLATE_NAME="dapper.template"
        BLOCKCHAIN_CONFIG_NAME="dapper.yml"
        ;;
    algorand)
        echo -n "algorand"
        BLOCKCHAIN_CONFIG_TEMPLATE_NAME="algorand.template"
        BLOCKCHAIN_CONFIG_NAME="algorand.yml"
        ;;
    *)
        echo -e "MA_BLOCKCHAIN environment variable must be set to one of: '${supported_blockchains[*]}'. Exiting."
        echo -e "Node type not supported: $MA_BLOCKCHAIN. Exiting."
        exit 1
        ;;
esac

echo -e "Metrika agent installation started for: ${MA_BLOCKCHAIN}"

BLOCKCHAIN=$MA_BLOCKCHAIN
APP_NAME=metrikad
BIN_NAME=metrikad-$BLOCKCHAIN
MA_USER=metrikad
MA_GROUP=$MA_USER
APP_METADATA_DIR="/etc/$APP_NAME"
INSTALL_DIR="/opt/$APP_NAME"
KNOWN_DISTRIBUTION="(Debian|Ubuntu)"
CONFIG_NAME="agent.yml"
AGENT_DOWNLOAD_URL="http://0.0.0.0:8000/$BIN_NAME"
AGENT_CONFIG_DOWNLOAD_URL="http://0.0.0.0:8000/internal/pkg/global/$CONFIG_NAME"

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

function create_systemd_service {
    $sudo_cmd systemctl --no-pager status -l $BIN_NAME.service || echo "Systemd service already exists, would you like to re-create it? (y/n)"
    # read $ans
    # check ans == 'y'

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
ExecStart=/usr/bin/env $INSTALL_DIR/$BIN_NAME

[Install]
WantedBy=multi-user.target
EOF
)
    log_info "\033[1;32m $service ${end}"
    echo "$service" | $sudo_cmd tee /lib/systemd/system/$BIN_NAME.service
    cd /etc/systemd/system
    $sudo_cmd ln -s /lib/systemd/system/$BIN_NAME.service $BIN_NAME.service || true
    $sudo_cmd systemctl daemon-reload
    $sudo_cmd systemctl --no-pager status -l $BIN_NAME.service
    $sudo_cmd systemctl enable $BIN_NAME.service
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

    echo "Creating system group(user): $MA_GROUP($MA_USER)"
    getent passwd "$MA_USER" >/dev/null || \
        $sudo_cmd adduser --system --group --home $INSTALL_DIR --shell /sbin/nologin "$MA_USER" && \
        { $sudo_cmd usermod -L "$MA_USER" || \
            log_warn "Cannot lock the 'metrika-agent' user account"; }

    echo "Preparing agent metadata directory: $APP_METADATA_DIR"
    $sudo_cmd mkdir -p $APP_METADATA_DIR/configs
    $sudo_cmd chown -R $MA_GROUP:$MA_USER $APP_METADATA_DIR

    # TODO download_binary
    echo "Downloading agent binary"
    $sudo_cmd mkdir -p $INSTALL_DIR
    # wget --quiet -O "$BIN_NAME" "$AGENT_DOWNLOAD_URL"
    # wget --quiet -O "$CONFIG_NAME" "$AGENT_CONFIG_DOWNLOAD_URL"

    echo "Installing agent..."
    $sudo_cmd cp -t $INSTALL_DIR "$BIN_NAME"
    $sudo_cmd cp -t $APP_METADATA_DIR $CONFIG_NAME
    $sudo_cmd cp -t $APP_METADATA_DIR/configs configs/$BLOCKCHAIN_CONFIG_TEMPLATE_NAME

    create_systemd_service

    # TODO
    # enable the agent systemd service
    # finally start the service
else
    # macOS
    goodbye "Distribution not supported: $DISTRIBUTION"
fi
