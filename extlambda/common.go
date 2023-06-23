package extlambda

import (
	"github.com/steadybit/action-kit/go/action_kit_api/v2"
	"github.com/steadybit/extension-kit/extutil"
)

const (
	lambdaTargetID   = "com.github.steadybit.extension_aws.lambda"
	lambdaTargetIcon = "data:image/svg+xml;base64,PHN2ZyB3aWR0aD0iMjQiIGhlaWdodD0iMjQiIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgeG1sbnM9Imh0dHA6Ly93d3cudzMub3JnLzIwMDAvc3ZnIj4KPHBhdGggZmlsbC1ydWxlPSJldmVub2RkIiBjbGlwLXJ1bGU9ImV2ZW5vZGQiIGQ9Ik01Ljk3MjI5IDcuMTAzODRWM0gxMS42OTMzTDE4LjQ0NjMgMTYuOTIwNUgyMC4yNDIzVjIxSDE1LjYxMDJMOC44NjUyOSA3LjEwMzg0SDUuOTcyMjlaTTEwLjQ1ODYgMTUuMjU1TDguMDQ1MiAxMC4yODM0TDMgMjAuOTY1NEg3LjgzODMzTDEwLjQ1ODYgMTUuMjU1WiIgZmlsbD0iIzFEMjYzMiIvPgo8L3N2Zz4K"
)

var (
	lambdaTargetSelection = action_kit_api.TargetSelection{
		TargetType: lambdaTargetID,
		SelectionTemplates: extutil.Ptr([]action_kit_api.TargetSelectionTemplate{
			{
				Label: "by function name",
				Query: "aws.lambda.function-name=\"\"",
			},
		}),
	}
)
