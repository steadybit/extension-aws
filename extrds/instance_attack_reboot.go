// SPDX-License-Identifier: MIT
// SPDX-FileCopyrightText: 2022 Steadybit GmbH

package extrds

import (
	"fmt"
	"github.com/steadybit/attack-kit/go/attack_kit_api"
	"github.com/steadybit/extension-aws/utils"
	"net/http"
)

func RegisterRdsAttackHandlers() {
	utils.RegisterHttpHandler("/rds/instance/attack/reboot", utils.GetterAsHandler(getRebootInstanceAttackDescription))
	utils.RegisterHttpHandler("/rds/instance/attack/reboot/prepare", prepareInstanceReboot)
	utils.RegisterHttpHandler("/rds/instance/attack/reboot/start", startInstanceReboot)

}

func getRebootInstanceAttackDescription() attack_kit_api.AttackDescription {
	return attack_kit_api.AttackDescription{
		Id:          fmt.Sprintf("%s.reboot", rdsTargetId),
		Label:       "reboot instance",
		Description: "Reboot a single database instance",
		Version:     "1.0.0",
		Icon:        attack_kit_api.Ptr(rdsIcon),
		TargetType:  rdsTargetId,
		Category:    attack_kit_api.Resource,
		// TODO
		TimeControl: attack_kit_api.INSTANTANEOUS,
		Parameters: []attack_kit_api.AttackParameter{
			// TODO
			{
				Label:        "Duration",
				Name:         "duration",
				Type:         "duration",
				Advanced:     attack_kit_api.Ptr(false),
				Required:     attack_kit_api.Ptr(true),
				DefaultValue: attack_kit_api.Ptr("30s"),
			},
			{
				Label:       "Consumer Username or ID",
				Name:        "consumer",
				Description: attack_kit_api.Ptr("You may optionally define for which Kong consumer the traffic should be impacted."),
				Type:        "string",
				Advanced:    attack_kit_api.Ptr(false),
				Required:    attack_kit_api.Ptr(false),
			},
			{
				Label:        "Message",
				Name:         "message",
				Type:         "string",
				Advanced:     attack_kit_api.Ptr(true),
				DefaultValue: attack_kit_api.Ptr("Error injected through the Steadybit Kong extension (through the request-termination Kong plugin)"),
			},
			{
				Label:        "HTTP status code",
				Name:         "status",
				Type:         "integer",
				Advanced:     attack_kit_api.Ptr(true),
				DefaultValue: attack_kit_api.Ptr("500"),
			},
		},
		Prepare: attack_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/rds/instance/attack/reboot/prepare",
		},
		Start: attack_kit_api.MutatingEndpointReference{
			Method: "POST",
			Path:   "/rds/instance/attack/reboot/start",
		},
	}
}

func prepareInstanceReboot(w http.ResponseWriter, r *http.Request, _ []byte) {
	utils.WriteBody(w, "TODO")
}

func startInstanceReboot(w http.ResponseWriter, r *http.Request, _ []byte) {
	utils.WriteBody(w, "TODO")
}
