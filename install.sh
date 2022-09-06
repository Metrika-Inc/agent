#!/usr/bin/env bash

PLATFORM_API_KEY=$MA_API_KEY
APP_NAME=metrikad
BIN_NAME=metrikad-$MA_BLOCKCHAIN
MA_USER=metrikad
MA_GROUP=$MA_USER
APP_METADATA_DIR="/etc/$APP_NAME"
APP_INSTALL_DIR="/opt/$APP_NAME"
KNOWN_DISTRIBUTION="(Scientific Linux|Linux Mint|openSUSE|CentOS|Arch|Debian|Ubuntu|Pop\!_OS|Fedora|Red Hat)"
AGENT_CONFIG_NAME="agent.yml"
INSTALLER_VERSION="0.1"
SUPPORTED_BLOCKCHAINS=("flow")
SUPPORTED_ARCHS=("arm64 x86_64")
LOGFILE="metrikad-install.log"
LATEST_RELEASE="0.0.0"
IS_UPDATABLE=-1
UPGRADE_REQUESTED=0
NO_DOCKER_GRP_REQUESTED=0
HAS_SYSTEMD=0
INSTALL_ID=$(date +%s)

function goodbye {
	echo -e ""
	log_error "${1}"
	log_ok "Help improve Metrika Agent! Please open an issue and attach /tmp/metrika-install-${INSTALL_ID}/metrika-install.log."
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

function check_for_systemd {
	HAS_SYSTEMD=0
	if [[ $(ps --no-headers -o comm 1) == "systemd" ]]; then
		HAS_SYSTEMD=1
	fi
}

function usage {
	cat <<EOF
usage: install.sh [options]
options:
  --upgrade            Upgrade the Metrika agent to the latest version.
  --reinstall          Reinstall/refresh the Metrika Agent installation.
  --uninstall          Stop and remove the Metrika Agent.
  --purge              Stop, remove the Metrika Agent, including any agent configuration/data.
  --no-docker-grp	   Do NOT add to the system docker group (requires docker proxy for containerized nodes!)
EOF
}

###lib start##

logo=$(echo "${logo_gz}" | base64 -d 2>/dev/null | gzip -d 2>/dev/null)
function print_header {
	if [ -z "$PS1" ]; then
		echo -e "\n${logo}"
		cat <<EOF
---------------------------
Metrika Agent Installer ${INSTALLER_VERSION}
---------------------------
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
	echo -e "\033[1;33m 🟠  ${1}${end}"
}

function log_info {
	echo -e "\033[34m 🔵  ${1}${end}"
}

function log_ok {
	echo -e "\033[1;32m 🟢  ${1}${end}"
}

function log_error {
	echo >&2 -e "\033[1;31;40m 🔴  ${1}${end}"
}

function stop_service {
	log_info "Stopping the agent, this might take few seconds..."

	$sudo_cmd systemctl stop -l "$BIN_NAME" || true
	$sudo_cmd systemctl disable -l "$BIN_NAME" || true
}

# from https://stackoverflow.com/a/4025065/325548
function verlte {
	local -n outvar=$3
	printf '%s\n%s' "$1" "$2" | sort -C -V
	outvar=$?
}

function determine_latest_version {
	local -n outvar=$1
	log_info "Determining the latest version of the Metrika agent..."
	gh_response="$(curl -s -H "Accept: application/vnd.github+json" https://api.github.com/repos/Metrika-Inc/agent/releases/latest)"
	if [ -n "$gh_response" ] && ! echo "$gh_response" | grep -qi "not found"; then
		LATEST_RELEASE=$(echo "${gh_response} " | grep "tag_name" | cut -f 2 -d ":" | tr -d 'v",' | xargs)
		log_info "Latest Release: ${LATEST_RELEASE}"
		outvar=${LATEST_RELEASE}
	else
		goodbye "Could not determine the latest version of the Metrika Agent. Ensure your system can connect to Github." 80
	fi
}

function check_can_update {
	local newer_version_out=0
	determine_latest_version LATEST_RELEASE
	if test -f "${APP_INSTALL_DIR}/${BIN_NAME}"; then
		current_release="$(${APP_INSTALL_DIR}/${BIN_NAME} --version | tr -d 'v')" || "0.0.0"
		log_info "Latest version: ${LATEST_RELEASE}, Current version: ${current_release}."
		verlte "${LATEST_RELEASE}" "${current_release}" newer_version_out
		IS_UPDATABLE=$newer_version_out
	else
		IS_UPDATABLE=1
	fi
}

function download_agent {
	if [[ -z "${MA_AGENT_DOWNLOAD_URL}" ]]; then
		# This is not a custom install. We'll check and get the binary from Github.
		case $IS_UPDATABLE in
		1)
			# TODO(cosmix): add the architecture here when we add multiarch support.
			download_url=$(echo "${gh_response}" | grep "url" | grep "browser_download_url" | grep "${MA_BLOCKCHAIN}" | cut -f 4 -d "\"" | tr -d '",' | xargs)
			log_info "Downloading the latest version (${LATEST_RELEASE}) of the Metrika Agent for ${MA_BLOCKCHAIN} from GitHub ${download_url}"
			if ! curl -s --output "metrika_agent_${MA_BLOCKCHAIN}_latest.tar.gz" -f -L -H "Accept: application/octet-stream" $download_url; then
				goodbye "Failed downloading the latest version of the Metrika agent." 60
			fi
			if ! tar -xvf "metrika_agent_${MA_BLOCKCHAIN}_latest.tar.gz"; then
				goodbye "Failed extracting the new version of the agent." 70
			fi

			if [ $UPGRADE_REQUESTED -ne 1 ]; then
				log_info "Downloading additional configuration for the Metrika agent."
				mkdir configs
				if ! curl -s https://raw.githubusercontent.com/Metrika-Inc/agent/v${LATEST_RELEASE}/configs/agent.yml -o configs/${AGENT_CONFIG_NAME}; then
					goodbye "Failed downloading agent default configuration and templates for ${MA_BLOCKCHAIN}. Try again later." 61
				fi
				if ! curl -s https://raw.githubusercontent.com/Metrika-Inc/agent/v${LATEST_RELEASE}/configs/${BLOCKCHAIN_CONFIG_TEMPLATE_NAME} -o "configs/${BLOCKCHAIN_CONFIG_TEMPLATE_NAME}"; then
					goodbye "Failed downloading agent default configuration and templates for ${MA_BLOCKCHAIN}. Try again later." 62
				fi
			fi
			;;
		2 | 0)
			goodbye "You already have the latest version of Metrika Agent installed (or are running a custom version!)" 50
			;;
		*)
			goodbye "Could not properly determine agent versions." 30
			;;
		esac
	else
		# ignore checks and download from the override URL
		if ! curl -s --output "metrika_agent_${MA_BLOCKCHAIN}_latest.tar.gz" -f -L -H "Accept: application/octet-stream" "${MA_AGENT_DOWNLOAD_URL}"; then
			goodbye "MA_AGENT_DOWNLOAD_URL is set. Failed downloading the latest version of the Metrika agent from that URL." 63
		fi
		if ! tar -xvf "metrika_agent_${MA_BLOCKCHAIN}_latest.tar.gz"; then
			goodbye "Failed extracting the new version of the agent." 70
		fi
	fi
}

function sanity_check {

	# CURL
	test -x "$(which curl)" || goodbye "curl is missing from your system. Please install it and try again." 10
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
		goodbye "Unsupported architecture. Metrika Agent currently supports: '${SUPPORTED_ARCHS[*]}'" 20
		;;
	esac

       # TODO: remove MA_PLATFORM dependency.
       # MA_PLATFORM envvar
       if [[ -z "${PLATFORM_ADDR}" ]]; then
           if [[ -z "${MA_PLATFORM}" ]]; then
               goodbye "MA_PLATFORM environment variable must be set'. Goodbye." 100
           fi
           PLATFORM_ADDR=${MA_PLATFORM}
       fi

	# MA_BLOCKCHAIN envvar
	if [[ -z "${MA_BLOCKCHAIN}" ]]; then
		goodbye "MA_BLOCKCHAIN environment variable must be set to one of: '${SUPPORTED_BLOCKCHAINS[*]}'. Goodbye." 2
	fi

	case $MA_BLOCKCHAIN in
	flow)
		true
		;;
	*)
		log_warn "Node type not currently supported: $MA_BLOCKCHAIN. Please find us on Discord or email us at support@metrika.co if you want to see $MA_BLOCKCHAIN supported!"
		goodbye "The MA_BLOCKCHAIN envvar must be set to one of: '${SUPPORTED_BLOCKCHAINS[*]}'" 2
		;;
	esac

	# MA_API_KEY envvar
	if [[ -z "${MA_API_KEY}" ]]; then
		goodbye "MA_API_KEY environment variable must be set before running the installation script. Goodbye." 3
	fi

	# Linux check.
	if [[ $(uname -s) != "Linux" ]]; then
		goodbye "Metrika Agent currently only supports GNU/Linux operating systems." 7
	fi

	# systemd check.
	check_for_systemd
	if [ $HAS_SYSTEMD -ne 1 ]; then
		goodbye "Metrika Agent currently requires systemd to be present on your system." 4
	fi

	# distro check.
	DISTRIBUTION=$(lsb_release -d 2>/dev/null | grep -Eo "$KNOWN_DISTRIBUTION" ||
		grep -Eo "$KNOWN_DISTRIBUTION" /etc/issue 2>/dev/null ||
		grep -Eo "$KNOWN_DISTRIBUTION" /etc/Eos-release 2>/dev/null ||
		grep -m1 -Eo "$KNOWN_DISTRIBUTION" /etc/os-release 2>/dev/null ||
		uname -s)
	test -n "$DISTRIBUTION" || log_warn >&2 "Could not detect your distribution. Your mileage may vary."

	# Other OS check (dev)
	if [ "$DISTRIBUTION" == "Darwin" ]; then
		goodbye "Metrika Agent does not support macOS just yet." 40
	fi

	if [[ "$DISTRIBUTION" =~ .*BSD.* ]]; then
		goodbye "Metrika Agent does not support *BSD just yet." 41
	fi
}

function purge {
	stop_service

	if [[ -n $APP_METADATA_DIR ]]; then
		# remove metadata
		$sudo_cmd rm -rf $APP_METADATA_DIR
	fi

	if [[ -n $APP_INSTALL_DIR ]]; then
		# remove installation directory
		$sudo_cmd rm -rf $APP_INSTALL_DIR
	fi

	# remove user artifacts
	$sudo_cmd userdel $MA_USER || true
	$sudo_cmd groupdel $MA_GROUP || true
}

function uninstall {
	stop_service

	$sudo_cmd rm -f "/lib/systemd/system/$BIN_NAME.service"
	$sudo_cmd rm -f "$APP_INSTALL_DIR/$BIN_NAME"
	$sudo_cmd userdel $MA_USER || true
}

function check_existing_install {
	if $sudo_cmd systemctl --no-pager list-units --full -all | grep -F "$BIN_NAME".service; then
		# agent is installed.
		if [ $UPGRADE_REQUESTED -ne 1 ]; then
			goodbye "The Metrika Agent is already installed. Please use --upgrade to upgrade your installation." 51
		fi
	fi
}

function create_systemd_service {
	log_info "Creating systemd service..."
	service=$(
		envsubst <<EOF
[Unit]
Description=Metrika Agent ($MA_BLOCKCHAIN)
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
	$sudo_cmd bash -c "echo \"$service\" > \"/lib/systemd/system/$BIN_NAME.service\""
	cd /etc/systemd/system || goodbye "Could not change to /etc/systemd/system." 6
	$sudo_cmd systemctl daemon-reload
	$sudo_cmd systemctl enable "$BIN_NAME.service"
	$sudo_cmd systemctl --no-pager start -l "$BIN_NAME.service"
	$sudo_cmd systemctl --no-pager status -l "$BIN_NAME.service"
}

function install_agent {
	log_info "Installing agent..."
	$sudo_cmd cp -t $APP_INSTALL_DIR "$BIN_NAME"
	$sudo_cmd chown -R $MA_USER:$MA_GROUP $APP_INSTALL_DIR

	# do not download configuration when upgrading. The config upgrade path has to be handled
	# by the agent itself.
	if [ $UPGRADE_REQUESTED -ne 1 ]; then
		log_warn "This is a new installation. Configuration files will be reset to defaults."
		$sudo_cmd cp -t $APP_METADATA_DIR/configs configs/$AGENT_CONFIG_NAME
		$sudo_cmd sed -i "s/<api_key>/$PLATFORM_API_KEY/g" $APP_METADATA_DIR/configs/$AGENT_CONFIG_NAME
		$sudo_cmd sed -i "s/<platform_addr>/$PLATFORM_ADDR/g" $APP_METADATA_DIR/configs/$AGENT_CONFIG_NAME
		$sudo_cmd cp -t $APP_METADATA_DIR/configs configs/"$BLOCKCHAIN_CONFIG_TEMPLATE_NAME"
	fi
}

function create_directories {
	log_info "Preparing agent installation directories: $APP_INSTALL_DIR, $APP_METADATA_DIR"
	$sudo_cmd mkdir –m 0755 -p $APP_METADATA_DIR/configs
	$sudo_cmd chown -R $MA_USER:$MA_GROUP $APP_METADATA_DIR
	$sudo_cmd mkdir -p $APP_INSTALL_DIR
	$sudo_cmd chown -R $MA_USER:$MA_GROUP $APP_INSTALL_DIR
}

function create_users_and_groups {
	log_info "Creating system group(user): $MA_GROUP($MA_USER)"
	getent passwd "$MA_USER" >/dev/null ||
		$sudo_cmd adduser --system --group --home $APP_INSTALL_DIR --shell /sbin/nologin "$MA_USER" &&
		{ $sudo_cmd usermod -L "$MA_USER" ||
			log_warn "Cannot lock the 'metrika-agent' user account"; }

	if [ $NO_DOCKER_GRP_REQUESTED -ne 1 ]; then
		$sudo_cmd usermod -aG docker $MA_USER
	else
		log_warn "NOT adding ${MA_USER} to the docker group. For containerized nodes, you WILL need to have a docker proxy running on the host to allow the metrika agent to retrieve data!"

	fi
}

###lib end##

# ----------------------------------
# -- Mainline program starts here --
# ----------------------------------
print_header


installer_dir="/tmp/metrikad-install-${INSTALL_ID}"
mkdir "${installer_dir}"
cd "${installer_dir}" || goodbye "Could not change to the installation directory ${installer_dir}" 5

# Set up logfile.
pipe=/tmp/$$.tmp
mknod $pipe p
tee <$pipe $LOGFILE &
exec 1>&-
exec 1>$pipe 2>&1
trap 'rm -f $pipe' EXIT


for arg in "$@"; do
	case "$arg" in
	"--upgrade")
		UPGRADE_REQUESTED=1
		;;
	"--no-docker-grp")
		NO_DOCKER_GRP_REQUESTED=1
		;;
	"--reinstall")
		uninstall
		;;
	"--uninstall")
		log_info "Uninstalling any existing Metrika Agent installation."
		uninstall
		exit 0
		;;
	"--purge")
		log_warn "Purging any existing Metrika Agent installation. This WILL remove all traces of an existing Metrika Agent installation from your system!"
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

if [ -z $MA_AGENT_DOWNLOAD_URL ]; then
	check_can_update $IS_UPDATABLE
fi

BLOCKCHAIN_CONFIG_TEMPLATE_NAME="${MA_BLOCKCHAIN}.template"

log_info "METRIKA agent installation started for blockchain protocol node: '${MA_BLOCKCHAIN}'"

#
# Linux installation
#

check_existing_install

# ensure agent is downloaded _before_ doing anything on the system.
download_agent

# if the agent is downloaded ok, create users etc.
if [ $UPGRADE_REQUESTED -ne 1 ]; then
	create_users_and_groups
	create_directories
fi

# install the agent.
install_agent

if [ $UPGRADE_REQUESTED -ne 1 ]; then
	create_systemd_service
else
	$sudo_cmd systemctl restart "$BIN_NAME"
fi

log_info "Please wait..."
sleep 5 # Give the agent a few seconds to wake up.

# Check agent is alive.
if systemctl is-active --quiet "${BIN_NAME}"; then
	log_ok "The Metrika Agent is now running! Visit https://app.metrika.co to view its data!"
else
	log_warn "The Metrika Agent was installed but has not started yet. You can try starting it using: systemctl start ${BIN_NAME}."
fi
