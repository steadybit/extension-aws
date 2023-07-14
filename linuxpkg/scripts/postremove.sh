#!/bin/sh -e

#
# Copyright 2023 steadybit GmbH. All rights reserved.
#

service_name="steadybit-extension-aws"
# decide if we should use SystemD or init/upstart
use_systemctl="True"
if ! command -V systemctl >/dev/null 2>&1; then
  use_systemctl="False"
fi

remove() {
  if [ "${use_systemctl}" = "True" ]; then
    systemctl mask "$service_name" || :
  fi
}

purge() {
  if [ "${use_systemctl}" = "True" ]; then
    if systemctl is-enabled --quiet "$service_name"; then
      systemctl disable "$service_name" || :
    fi
    systemctl unmask "$service_name" || :
  fi
}

upgrade() {
  :
}

action="$1"

case "$action" in
"0" | "purge")
  purge
  ;;
"remove")
  remove
  ;;
"1" | "upgrade")
  upgrade
  ;;
*)
  # $1 == version being installed on Alpine
  remove
  ;;
esac
