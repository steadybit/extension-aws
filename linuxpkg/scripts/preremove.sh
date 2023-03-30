#!/bin/sh -e

#
# Copyright 2023 steadybit GmbH. All rights reserved.
#

# decide if we should use SystemD or init/upstart
use_systemctl="True"
if ! command -V systemctl >/dev/null 2>&1; then
  use_systemctl="False"
fi

# stop the service in the proper way for this platform
if [ "${use_systemctl}" = "False" ]; then
  if service steadybit-extension-aws status 2>/dev/null; then
    service steadybit-extension-aws stop
  fi
else
  if systemctl is-active --quiet steadybit-extension-aws; then
    systemctl stop steadybit-extension-aws
  fi
  if systemctl is-enabeld --quiet steadybit-extension-aws; then
    systemctl disable steadybit-extension-aws
  fi
fi
