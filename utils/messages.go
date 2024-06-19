// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2024 Steadybit GmbH
package utils

import (
	"fmt"
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
)

func AppendInfof(messages *action_kit_api.Messages, format string, args ...interface{}) *action_kit_api.Messages {
	return AppendMessagef(messages, action_kit_api.Info, format, args...)
}

func AppendWarnf(messages *action_kit_api.Messages, format string, args ...interface{}) *action_kit_api.Messages {
	return AppendMessagef(messages, action_kit_api.Warn, format, args...)
}

func AppendMessagef(messages *action_kit_api.Messages, level action_kit_api.MessageLevel, format string, args ...interface{}) *action_kit_api.Messages {
	if messages == nil {
		messages = &action_kit_api.Messages{}
	}
	return extutil.Ptr(append(*messages, action_kit_api.Message{Level: &level, Message: fmt.Sprintf(format, args...)}))
}
