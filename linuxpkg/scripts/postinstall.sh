#!/bin/sh -e

#
# Copyright 2023 steadybit GmbH. All rights reserved.
#

# decide if we should use SystemD or init/upstart
service_name="steadybit-extension-aws"
use_systemctl="True"
if ! command -V systemctl >/dev/null 2>&1; then
  use_systemctl="False"
fi

cleanup() {
  # remove files that were not needed on this platform / system
  if [ "${use_systemctl}" = "False" ]; then
    rm -f "/usr/lib/systemd/system/$service_name.service"
  else
    rm -f "/etc/chkconfig/$service_name"
    rm -f "/etc/init.d/$service_name"
  fi
}

cleanInstall() {
  if [ -n "$STEADYBIT_LOG_LEVEL" ]; then
    sed -i "s/^STEADYBIT_LOG_LEVEL=.*/STEADYBIT_LOG_LEVEL=$(echo "$STEADYBIT_LOG_LEVEL" | sed 's,/,\\/,g')/" /etc/steadybit/extension-aws
  fi

  if [ -n "$AWS_REGION" ]; then
    sed -i "s/^AWS_REGION=.*/AWS_REGION=$(echo "$AWS_REGION" | sed 's,/,\\/,g')/" /etc/steadybit/extension-aws
  fi

  # enable the service in the proper way for this platform
  if [ "${use_systemctl}" = "False" ]; then
    if command -V chkconfig >/dev/null 2>&1; then
      chkconfig --add "$service_name"
    fi

    service "$service_name" restart || :
  else
    systemctl daemon-reload || :
    systemctl unmask "$service_name" || :
    systemctl preset "$service_name" || :
    systemctl enable "$service_name" || :
    systemctl restart "$service_name" || :
  fi

}

upgrade() {
  # enable the service in the proper way for this platform
  if [ "${use_systemctl}" = "False" ]; then
    if service "$service_name" status 2>/dev/null; then
      service "$service_name" restart
    fi
  else
    if systemctl is-active --quiet "$service_name"; then
      systemctl daemon-reload
      systemctl restart "$service_name"
    fi
  fi
}

#check if this is a clean install or an upgrade
action="$1"
if [ "$1" = "configure" ] && [ -z "$2" ]; then
  # Alpine linux does not pass args, and deb passes $1=configure
  action="install"
elif [ "$1" = "configure" ] && [ -n "$2" ]; then
  # deb passes $1=configure $2=<current version>
  action="upgrade"
fi

case "$action" in
"1" | "install")
  cleanInstall
  ;;
"2" | "upgrade")
  upgrade
  ;;
*)
  # $1 == version being installed on Alpine
  cleanInstall
  ;;
esac

cleanup
