#!/bin/sh -e

#
# Copyright 2023 steadybit GmbH. All rights reserved.
#

# decide if we should use SystemD or init/upstart
use_systemctl="True"
systemd_version=0
if ! command -V systemctl >/dev/null 2>&1; then
  use_systemctl="False"
else
  systemd_version=$(systemctl --version | head -1 | sed 's/systemd //g')
fi

cleanup() {
  # remove files that were not needed on this platform / system
  if [ "${use_systemctl}" = "False" ]; then
    rm -f /usr/lib/systemd/system/steaybit-extension-aws.service
  else
    rm -f /etc/chkconfig/steaybit-extension-aws
    rm -f /etc/init.d/steaybit-extension-aws
  fi
}

cleanInstall() {
  if [ -n "$STEADYBIT_LOG_LEVEL" ]; then
    sed -i "s/^$STEADYBIT_LOG_LEVEL=.*/$STEADYBIT_LOG_LEVEL=$(echo "$$STEADYBIT_LOG_LEVEL" | sed 's,/,\\/,g')/" /etc/steadybit/extension-aws
  fi

  # enable the service in the proper way for this platform
  if [ "${use_systemctl}" = "False" ]; then
    if command -V chkconfig >/dev/null 2>&1; then
      chkconfig --add steadybit-extension-aws
    fi

    service steadybit-extension-aws restart || :
  else
    systemctl daemon-reload || :
    systemctl unmask steadybit-extension-aws || :
    systemctl preset steadybit-extension-aws || :
    systemctl enable steadybit-extension-aws || :
    systemctl restart steadybit-extension-aws || :
  fi

}

upgrade() {
  # enable the service in the proper way for this platform
  if [ "${use_systemctl}" = "False" ]; then
    if service steadybit-extension-aws status 2>/dev/null; then
      service steadybit-extension-aws restart
    fi
  else
    if systemctl is-active --quiet steadybit-extension-aws; then
      systemctl daemon-reload
      systemctl restart steadybit-extension-aws
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
