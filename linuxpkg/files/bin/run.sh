#!/bin/bash

#
# Copyright 2023 steadybit GmbH. All rights reserved.
#

EXTENSION_BASE="/opt/steadybit/extension-aws"
EXEC_NAME="steadybit-extension-aws"


echo "Starting steadybit extension-aws ..."
exec $EXTENSION_BASE/$EXEC_NAME "$@"
