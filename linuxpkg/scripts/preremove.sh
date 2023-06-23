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

# stop the service in the proper way for this platform
if [ "${use_systemctl}" = "False" ]; then
  if service "$service_name" status 2>/dev/null; then
    service "$service_name" stop
  fi
else
  if systemctl is-active --quiet "$service_name"; then
    systemctl stop "$service_name"
  fi
  if systemctl is-enabled --quiet "$service_name"; then
    systemctl disable "$service_name"
  fi
fi
